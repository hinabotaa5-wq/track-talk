package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

func bornYearsAgo(n int) time.Time {
	now := time.Now()
	return time.Date(now.Year()-n, now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// day は日本時間の日付を返す。アプリ側も日本時間で判定するため、
// サーバーのタイムゾーン（CI は UTC）に引きずられないようにする。
func day(offset int) string {
	return time.Now().In(jst.Zone).AddDate(0, 0, offset).Format("2006-01-02")
}

// consultCoach は、1ヶ月プランを担当し、無料相談を受け付けているコーチを作る。
func consultCoach(t *testing.T, h http.Handler) (uuid.UUID, string) {
	t.Helper()
	id, tc, _ := planCoach(t, h, 5)
	if r := doJSON(t, h, "PATCH", "/v1/coach/me/services", tc, map[string]any{"service_types": []string{"video_review", "competition_support"}}); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	r := doJSON(t, h, "PUT", "/v1/coach/me/support-consult", tc, map[string]any{
		"accepting": true, "events": []string{"100m"}, "description": "スプリントのフォームと大会に向けた調整を見ます。",
		"price_min_yen": 5000, "price_max_yen": 12000,
	})
	if r.Code != 200 {
		t.Fatalf("settings %d %s", r.Code, r.Body.String())
	}
	return id, tc
}

func consultAnswersBody() map[string]any {
	return map[string]any{
		"goal_decided": true, "competition_name": "秋季記録会", "competition_date": day(40),
		"event": "100m", "goals": []string{"pb"}, "target_record": "12.00",
		"personal_best": "12.50", "pb_when": "within_3m", "form": "good",
		"club": true, "has_coach": false, "menu_by": "self", "practice_days": 4, "session_minutes": 90,
		"track_access": "yes", "weights_access": "sometimes", "can_film": "yes",
		"injury_now": false, "injury_past_year": "none", "rest_period": false,
		"focus": []string{"start", "acceleration"}, "main_worry": "スタートで出遅れます",
		"has_video": true,
		"want_menu": true, "want_video_reply": "often", "want_peak": true, "start_timing": "asap",
		"question_to_coach": "週に何回くらい動画を送ればよいですか",
	}
}

// 提案の期間は開始日から1ヶ月（30日・両端を含む）でサーバーが組み立てる。目標の大会は名前だけを目標として添える。
func proposalBody() map[string]any {
	return map[string]any{
		"competition_name": "秋季記録会", "support_start_date": day(1),
		"agree_report": true, "agree_detection": true,
		"call_count": 1, "call_minutes": 30, "price_yen": 9000, "content_note": "週1回の返答とメニュー見直しです",
		"answer_to_question": "週に1回で十分です", "valid_until": day(7),
	}
}

type consultView struct {
	Consult struct {
		ID         string  `json:"id"`
		Status     string  `json:"status"`
		Minor      bool    `json:"minor"`
		ContractID *string `json:"contract_id"`
		Proposal   *struct {
			PriceYen         int    `json:"price_yen"`
			FeePercent       int    `json:"fee_percent"`
			CoachReceivesYen int    `json:"coach_receives_yen"`
			DurationDays     int    `json:"duration_days"`
			CompetitionDate  string `json:"competition_date"`
		} `json:"proposal"`
		Questions []struct {
			Answer *string `json:"answer"`
		} `json:"questions"`
	} `json:"consult"`
}

func decodeConsult(t *testing.T, body []byte) consultView {
	t.Helper()
	var v consultView
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func sendConsult(t *testing.T, h http.Handler, athlete string, coach uuid.UUID, answers map[string]any) string {
	t.Helper()
	r := doJSON(t, h, "POST", "/v1/support-consults", athlete, map[string]any{"coach_id": coach, "answers": answers})
	if r.Code != 201 {
		t.Fatalf("consult %d %s", r.Code, r.Body.String())
	}
	return decodeConsult(t, r.Body.Bytes()).Consult.ID
}

var consentBody = map[string]any{"agree_report": true, "agree_detection": true}

func TestSupportConsultFullFlow(t *testing.T) {
	h := setup(t)
	coach, tc := consultCoach(t, h)
	_, ta := demoUser(t, h, bornYearsAgo(20))

	// 入口は公開で見え、受付中。件数や残り枠は含まない。
	r := doJSON(t, h, "GET", "/v1/coaches/"+coach.String()+"/support-consult", "", nil)
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"status":"open"`) || strings.Contains(r.Body.String(), "capacity") || strings.Contains(r.Body.String(), "remaining") {
		t.Fatalf("public %d %s", r.Code, r.Body.String())
	}
	// コーチ一覧にも入口が出る。
	if r := doJSON(t, h, "GET", "/v1/coaches", "", nil); !strings.Contains(r.Body.String(), `"support_consult":{`) {
		t.Fatalf("list %s", r.Body.String())
	}
	// 提案用の入れ物のプランは、プラン管理と公開の一覧に出ない。
	if r := doJSON(t, h, "GET", "/v1/coach/me/plans", tc, nil); strings.Contains(r.Body.String(), "無料相談の提案") {
		t.Fatalf("template leaked: %s", r.Body.String())
	}

	id := sendConsult(t, h, ta, coach, consultAnswersBody())
	// 同じコーチへの相談は進行中は重ねられない。
	if r := doJSON(t, h, "POST", "/v1/support-consults", ta, map[string]any{"coach_id": coach, "answers": consultAnswersBody()}); r.Code != 409 {
		t.Fatalf("dup %d", r.Code)
	}
	// 選手は提案の前に受けられない。
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/accept", ta, consentBody); r.Code != 409 {
		t.Fatalf("early accept %d", r.Code)
	}
	// コーチが聞き直し → 選手が答える。
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/ask", tc, map[string]any{"question": "週末は練習できますか"}); r.Code != 200 {
		t.Fatalf("ask %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/propose", tc, proposalBody()); r.Code != 409 {
		t.Fatalf("propose while clarifying %d", r.Code)
	}
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/answer", ta, map[string]any{"answer": "土日は練習できます"}); r.Code != 200 {
		t.Fatalf("answer %d %s", r.Code, r.Body.String())
	}
	// 提案には開始日・有効期限・価格・手数料が入る。
	r = doJSON(t, h, "POST", "/v1/support-consults/"+id+"/propose", tc, proposalBody())
	if r.Code != 200 {
		t.Fatalf("propose %d %s", r.Code, r.Body.String())
	}
	v := decodeConsult(t, r.Body.Bytes())
	if v.Consult.Status != "proposed" || v.Consult.Proposal == nil || v.Consult.Proposal.PriceYen != 9000 || v.Consult.Proposal.FeePercent != 25 || v.Consult.Proposal.CoachReceivesYen != 6750 || v.Consult.Proposal.DurationDays != 30 {
		t.Fatalf("proposal %+v", v.Consult)
	}
	// 他人は相談を読めない。
	_, tother := demoUser(t, h, bornYearsAgo(20))
	if r := doJSON(t, h, "GET", "/v1/support-consults/"+id, tother, nil); r.Code != 404 {
		t.Fatalf("other %d", r.Code)
	}
	// 選手が受けると契約ができる。コーチの再引受は挟まず、デモ版（支払いなし）はそのまま始まる。中身は提案と一致する。
	r = doJSON(t, h, "POST", "/v1/support-consults/"+id+"/accept", ta, consentBody)
	if r.Code != 201 {
		t.Fatalf("accept %d %s", r.Code, r.Body.String())
	}
	ct := decodeContract(t, r.Body.Bytes())
	if ct.Status != "active" {
		t.Fatalf("contract %+v", ct)
	}
	var price, days int
	var service, name, desc string
	var compDate time.Time
	var videos int
	queryRowArgs(t, `SELECT price_yen, duration_days, service_type, competition_name, competition_date, plan_description, video_reviews FROM coaching_contracts WHERE id=$1`, []any{ct.ID}, &price, &days, &service, &name, &compDate, &desc, &videos)
	if price != 9000 || days != 30 || service != "competition_support" || name != "秋季記録会" || compDate.Format("2006-01-02") != day(30) || videos != 0 {
		t.Fatalf("snapshot %d %d %s %s %s %d", price, days, service, name, compDate, videos)
	}
	// 「何が返ってくるか」は標準形（動画への返答は無制限）が契約の説明に入る。
	if !strings.Contains(desc, "回数の制限なく") {
		t.Fatalf("standard content missing: %s", desc)
	}
	var coachConsent int
	queryRowArgs(t, `SELECT count(*)::int FROM coaching_contract_consents WHERE contract_id=$1 AND party='coach'`, []any{ct.ID}, &coachConsent)
	if coachConsent != 1 {
		t.Fatalf("coach consent %d", coachConsent)
	}
	var endsOK bool
	queryRowArgs(t, `SELECT ends_at IS NULL OR (ends_at AT TIME ZONE 'Asia/Tokyo')::date = ($2::date + 1) FROM coaching_contracts WHERE id=$1`, []any{ct.ID, day(30)}, &endsOK)
	if !endsOK {
		t.Fatal("ends_at is not the day after the 1-month period end")
	}
	// 契約が進行中のコーチへは、新しい無料相談を起こせない。
	if r := doJSON(t, h, "POST", "/v1/support-consults", ta, map[string]any{"coach_id": coach, "answers": consultAnswersBody()}); r.Code != 409 {
		t.Fatalf("consult during contract %d", r.Code)
	}
	// 追加相談は1ヶ月プランにはない。
	if r := doJSON(t, h, "GET", "/v1/coaching/contracts/"+ct.ID, ta, nil); r.Code != 200 {
		t.Fatalf("contract get %d", r.Code)
	}
}

func TestSupportConsultDeclineAndWithdrawAndReconsult(t *testing.T) {
	h := setup(t)
	coach, tc := consultCoach(t, h)
	_, ta := demoUser(t, h, bornYearsAgo(20))
	id := sendConsult(t, h, ta, coach, consultAnswersBody())
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/decline", tc, nil); r.Code != 200 {
		t.Fatalf("decline %d %s", r.Code, r.Body.String())
	}
	// 辞退のあとは、次の大会向けにもう一度相談できる（回数制限はない）。
	id2 := sendConsult(t, h, ta, coach, consultAnswersBody())
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id2+"/withdraw", ta, nil); r.Code != 200 {
		t.Fatalf("withdraw %d", r.Code)
	}
	id3 := sendConsult(t, h, ta, coach, consultAnswersBody())
	// 相談だけで申し込まなければ、契約は生まれない。
	var n int
	queryRowArgs(t, `SELECT count(*) FROM coaching_contracts WHERE requester_id=(SELECT athlete_id FROM support_consults WHERE id=$1)`, []any{id3}, &n)
	if n != 0 {
		t.Fatalf("contracts %d", n)
	}
	// 相手のコーチ以外は辞退できない。
	_, tother := consultCoach(t, h)
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id3+"/decline", tother, nil); r.Code != 404 {
		t.Fatalf("other coach decline %d", r.Code)
	}
}

func TestSupportConsultValidationAndContact(t *testing.T) {
	h := setup(t)
	coach, _ := consultCoach(t, h)
	_, ta := demoUser(t, h, bornYearsAgo(20))
	post := func(mutate func(map[string]any)) int {
		a := consultAnswersBody()
		mutate(a)
		return doJSON(t, h, "POST", "/v1/support-consults", ta, map[string]any{"coach_id": coach, "answers": a}).Code
	}
	for name, mutate := range map[string]func(map[string]any){
		"種目がコーチの担当外":  func(a map[string]any) { a["event"] = "400m" },
		"大会が過去":       func(a map[string]any) { a["competition_date"] = day(-3) },
		"自己ベストが数値でない": func(a map[string]any) { a["personal_best"] = "速い" },
		"怪我ありで部位なし":   func(a map[string]any) { a["injury_now"] = true; a["injury_note"] = "痛い" },
		"悩みに連絡先":      func(a map[string]any) { a["main_worry"] = "line id: abc_def 連絡ください http://x.example" },
		"目標が空":        func(a map[string]any) { a["goals"] = []string{} },
		"未定で時期なし": func(a map[string]any) {
			a["goal_decided"] = false
			delete(a, "competition_name")
			delete(a, "competition_date")
		},
	} {
		if code := post(mutate); code != 422 {
			t.Fatalf("%s: %d", name, code)
		}
	}
	// 未定なら時期の目安で送れる。
	if code := post(func(a map[string]any) {
		a["goal_decided"] = false
		a["timing_hint"] = "3_6m"
		delete(a, "competition_name")
		delete(a, "competition_date")
	}); code != 201 {
		t.Fatalf("undecided %d", code)
	}
}

func TestSupportConsultReceptionIsManualAndIgnoresCalendar(t *testing.T) {
	h := setup(t)
	coach, tc := consultCoach(t, h)
	_, ta := demoUser(t, h, bornYearsAgo(20))
	// カレンダーを閉じても、受付状態は変わらない（連動しない）。
	if r := doJSON(t, h, "PUT", "/v1/coach/me/availability", tc, map[string]any{"days": []map[string]any{{"date": day(0), "status": "unavailable"}, {"date": day(1), "status": "unavailable"}}}); r.Code != 200 {
		t.Fatalf("availability %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, "GET", "/v1/coaches/"+coach.String()+"/support-consult", "", nil); !strings.Contains(r.Body.String(), `"status":"open"`) {
		t.Fatalf("still open %s", r.Body.String())
	}
	a := consultAnswersBody()
	a["start_timing"] = "date"
	a["start_date"] = day(0)
	if r := doJSON(t, h, "POST", "/v1/support-consults", ta, map[string]any{"coach_id": coach, "answers": a}); r.Code != 201 {
		t.Fatalf("send with closed calendar day %d %s", r.Code, r.Body.String())
	}
	// コーチが手で受付停止中にすると、入口は「受付停止中」で、新しい相談は送れない。
	set := func(accepting bool) {
		if r := doJSON(t, h, "PUT", "/v1/coach/me/support-consult", tc, map[string]any{
			"accepting": accepting, "events": []string{"100m"}, "description": "スプリントのフォームと大会に向けた調整を見ます。",
			"price_min_yen": 5000, "price_max_yen": 12000,
		}); r.Code != 200 {
			t.Fatalf("settings %d %s", r.Code, r.Body.String())
		}
	}
	set(false)
	r := doJSON(t, h, "GET", "/v1/coaches/"+coach.String()+"/support-consult", "", nil)
	if !strings.Contains(r.Body.String(), `"status":"paused"`) || strings.Contains(r.Body.String(), "capacity") || strings.Contains(r.Body.String(), "remaining") {
		t.Fatalf("paused %s", r.Body.String())
	}
	_, ta2 := demoUser(t, h, bornYearsAgo(20))
	if r := doJSON(t, h, "POST", "/v1/support-consults", ta2, map[string]any{"coach_id": coach, "answers": consultAnswersBody()}); r.Code != 409 {
		t.Fatalf("paused send %d", r.Code)
	}
	set(true)
	if r := doJSON(t, h, "GET", "/v1/coaches/"+coach.String()+"/support-consult", "", nil); !strings.Contains(r.Body.String(), `"status":"open"`) {
		t.Fatalf("reopen %s", r.Body.String())
	}
}

func TestSupportConsultMinorGoesThroughGuardianFlow(t *testing.T) {
	h := setup(t)
	coach, tc := consultCoach(t, h)
	tm, gt := guardianMinor(t, h)
	id := sendConsult(t, h, tm, coach, consultAnswersBody())
	if v := decodeConsult(t, doJSON(t, h, "GET", "/v1/support-consults/"+id, tm, nil).Body.Bytes()); !v.Consult.Minor {
		t.Fatal("minor flag")
	}
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/propose", tc, proposalBody()); r.Code != 200 {
		t.Fatalf("propose %d %s", r.Code, r.Body.String())
	}
	countContracts := func() (n int) {
		queryRowArgs(t, `SELECT count(*)::int FROM coaching_contracts WHERE requester_id=(SELECT athlete_id FROM support_consults WHERE id=$1)`, []any{id}, &n)
		return
	}
	// 未成年本人の「申し込む」では契約ができない。保護者に送る前は、保護者のページにも出ない。
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/accept", tm, consentBody); r.Code != 409 || countContracts() != 0 {
		t.Fatalf("minor accept %d contracts=%d", r.Code, countContracts())
	}
	if r := doJSON(t, h, "GET", "/v1/guardian/"+gt+"/support-consults/"+id, "", nil); r.Code != 404 {
		t.Fatalf("guardian sees before send %d", r.Code)
	}
	// 保護者に提案を送る。まだ契約はできない。
	r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/send-to-guardian", tm, nil)
	if r.Code != 200 || decodeConsult(t, r.Body.Bytes()).Consult.Status != "sent_to_guardian" || countContracts() != 0 {
		t.Fatalf("send %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, "GET", "/v1/guardian/"+gt+"/support-consults", "", nil); r.Code != 200 || !strings.Contains(r.Body.String(), id) {
		t.Fatalf("guardian list %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, "GET", "/v1/guardian/"+gt+"/support-consults/"+id, "", nil); r.Code != 200 || !strings.Contains(r.Body.String(), `"price_yen":9000`) {
		t.Fatalf("guardian get %d %s", r.Code, r.Body.String())
	}
	// 他の子の保護者には見えない。
	_, gt2 := guardianMinor(t, h)
	if r := doJSON(t, h, "GET", "/v1/guardian/"+gt2+"/support-consults/"+id, "", nil); r.Code != 404 {
		t.Fatalf("other guardian %d", r.Code)
	}
	if r := doJSON(t, h, "POST", "/v1/guardian/"+gt+"/support-consults/"+id+"/consent", "", map[string]any{"agree_report": true}); r.Code != 422 {
		t.Fatalf("partial consent %d", r.Code)
	}
	// 保護者の同意で契約ができ、コーチの再引受なしに、デモ版（支払いなし）はそのまま始まる。
	r = doJSON(t, h, "POST", "/v1/guardian/"+gt+"/support-consults/"+id+"/consent", "", consentBody)
	if r.Code != 200 || countContracts() != 1 {
		t.Fatalf("guardian consent %d %s", r.Code, r.Body.String())
	}
	var status string
	var guardianConsents, coachConsents int
	queryRowArgs(t, `SELECT status FROM coaching_contracts WHERE requester_id=(SELECT athlete_id FROM support_consults WHERE id=$1)`, []any{id}, &status)
	queryRowArgs(t, `SELECT count(*)::int FROM coaching_contract_consents WHERE party='guardian' AND contract_id=(SELECT contract_id FROM support_consults WHERE id=$1)`, []any{id}, &guardianConsents)
	queryRowArgs(t, `SELECT count(*)::int FROM coaching_contract_consents WHERE party='coach' AND contract_id=(SELECT contract_id FROM support_consults WHERE id=$1)`, []any{id}, &coachConsents)
	if status != "active" || guardianConsents != 1 || coachConsents != 1 {
		t.Fatalf("contract status=%s guardian=%d coach=%d", status, guardianConsents, coachConsents)
	}
	if r := doJSON(t, h, "POST", "/v1/guardian/"+gt+"/support-consults/"+id+"/consent", "", consentBody); r.Code != 409 {
		t.Fatalf("double consent %d", r.Code)
	}
}

func TestSupportConsultMinorGuardianNeedsConsentAndCanDecline(t *testing.T) {
	h := setup(t)
	coach, tc := consultCoach(t, h)
	// 保護者が未同意なら、保護者に送れない。
	_, tm0 := demoUser(t, h, bornYearsAgo(15))
	id0 := sendConsult(t, h, tm0, coach, consultAnswersBody())
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id0+"/propose", tc, proposalBody()); r.Code != 200 {
		t.Fatalf("propose %d", r.Code)
	}
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id0+"/send-to-guardian", tm0, nil); r.Code != 409 {
		t.Fatalf("send without guardian %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id0+"/withdraw", tm0, nil); r.Code != 200 {
		t.Fatalf("withdraw %d", r.Code)
	}
	// 保護者が同意しない場合は、契約ができないまま終わる。
	tm, gt := guardianMinor(t, h)
	id := sendConsult(t, h, tm, coach, consultAnswersBody())
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/propose", tc, proposalBody()); r.Code != 200 {
		t.Fatalf("propose2 %d", r.Code)
	}
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/send-to-guardian", tm, nil); r.Code != 200 {
		t.Fatalf("send %d", r.Code)
	}
	if r := doJSON(t, h, "POST", "/v1/guardian/"+gt+"/support-consults/"+id+"/decline", "", nil); r.Code != 200 {
		t.Fatalf("decline %d %s", r.Code, r.Body.String())
	}
	var n int
	queryRowArgs(t, `SELECT count(*)::int FROM coaching_contracts WHERE requester_id=(SELECT athlete_id FROM support_consults WHERE id=$1)`, []any{id}, &n)
	if n != 0 {
		t.Fatalf("contracts %d", n)
	}
}

func TestSupportPlanHasNoAdditionalConsultation(t *testing.T) {
	h := setup(t)
	_, tc := consultCoach(t, h)
	body := nominationSupportTestBody()
	body["service_type"] = "competition_support"
	body["additional_consultation_price_yen"] = 600
	if r := doJSON(t, h, "POST", "/v1/coach/me/plans", tc, body); r.Code != 422 || !strings.Contains(r.Body.String(), "additional_consultation_price_yen") {
		t.Fatalf("support plan with additional price %d %s", r.Code, r.Body.String())
	}
}

func TestSupportConsultProposalValidationAndExpiry(t *testing.T) {
	h := setupConfig(t, func(c *config.Config) { c.CoachingTickSecret = "tick-secret" })
	coach, tc := consultCoach(t, h)
	_, ta := demoUser(t, h, bornYearsAgo(20))
	id := sendConsult(t, h, ta, coach, consultAnswersBody())
	bad := proposalBody()
	bad["price_yen"] = 1000
	bad["valid_until"] = day(60)
	bad["support_start_date"] = day(50)
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/propose", tc, bad); r.Code != 422 {
		t.Fatalf("bad proposal %d %s", r.Code, r.Body.String())
	}
	// 聞きたいことがある相談には、回答が必要。
	noAnswer := proposalBody()
	noAnswer["answer_to_question"] = ""
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/propose", tc, noAnswer); r.Code != 422 {
		t.Fatalf("no answer %d", r.Code)
	}
	// 期間は開始日から1ヶ月で決まり、有効期限は期間の終わりまで。開始日+30日より後の有効期限は受け付けない。
	late := proposalBody()
	late["valid_until"] = day(31)
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/propose", tc, late); r.Code != 422 {
		t.Fatalf("valid_until after period end %d", r.Code)
	}
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/propose", tc, proposalBody()); r.Code != 200 {
		t.Fatalf("propose %d", r.Code)
	}
	// 有効期限が過ぎた提案は受けられず、tick で閉じる。
	execSQLArgs(t, `UPDATE support_consult_proposals SET valid_until = now() - interval '1 minute' WHERE consult_id = $1`, id)
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/accept", ta, consentBody); r.Code != 409 {
		t.Fatalf("expired accept %d", r.Code)
	}
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"support_consults_expired":1`) {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	if v := decodeConsult(t, doJSON(t, h, "GET", "/v1/support-consults/"+id, ta, nil).Body.Bytes()); v.Consult.Status != "expired" {
		t.Fatalf("status %s", v.Consult.Status)
	}
	// コーチが 48 時間返事をしなかった相談も閉じる。
	id2 := sendConsult(t, h, ta, coach, consultAnswersBody())
	execSQLArgs(t, `UPDATE support_consults SET answer_by = now() - interval '1 minute' WHERE id = $1`, id2)
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id2+"/propose", tc, proposalBody()); r.Code != 409 {
		t.Fatalf("late propose %d", r.Code)
	}
	tick(t, h, "tick-secret")
	if v := decodeConsult(t, doJSON(t, h, "GET", "/v1/support-consults/"+id2, ta, nil).Body.Bytes()); v.Consult.Status != "expired" {
		t.Fatalf("status2 %s", v.Consult.Status)
	}
}

func TestSupportConsultSettingsRules(t *testing.T) {
	h := setup(t)
	_, tc, _ := planCoach(t, h, 5)
	body := map[string]any{"accepting": true, "events": []string{"100m"}, "description": "見ます"}
	// 1ヶ月プランを担当していなければ受け付けられない。
	if r := doJSON(t, h, "PUT", "/v1/coach/me/support-consult", tc, body); r.Code != 422 {
		t.Fatalf("no service %d %s", r.Code, r.Body.String())
	}
	doJSON(t, h, "PATCH", "/v1/coach/me/services", tc, map[string]any{"service_types": []string{"competition_support"}})
	body["price_min_yen"] = 9000
	body["price_max_yen"] = 5000
	if r := doJSON(t, h, "PUT", "/v1/coach/me/support-consult", tc, body); r.Code != 422 {
		t.Fatalf("price %d", r.Code)
	}
	body["price_max_yen"] = 12000
	if r := doJSON(t, h, "PUT", "/v1/coach/me/support-consult", tc, body); r.Code != 200 {
		t.Fatalf("ok %d %s", r.Code, r.Body.String())
	}
	// 受け入れ枠・上限に関する設定項目はない。
	if r := doJSON(t, h, "GET", "/v1/coach/me/support-consult", tc, nil); strings.Contains(r.Body.String(), "capacity") || strings.Contains(r.Body.String(), "limit") {
		t.Fatalf("capacity leaked %s", r.Body.String())
	}
	// 受付を止めると、公開の入口から消える。
	body["accepting"] = false
	doJSON(t, h, "PUT", "/v1/coach/me/support-consult", tc, body)
}
