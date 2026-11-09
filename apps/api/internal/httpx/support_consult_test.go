package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

func bornYearsAgo(n int) time.Time {
	now := time.Now()
	return time.Date(now.Year()-n, now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func day(offset int) string { return time.Now().AddDate(0, 0, offset).Format("2006-01-02") }

// consultCoach は、大会までプランを担当し、無料相談を受け付けているコーチを作る。
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

func proposalBody() map[string]any {
	return map[string]any{
		"competition_name": "秋季記録会", "support_start_date": day(1), "competition_date": day(40),
		"video_reviews": 4, "menu_updates": "weekly", "reply_formats": []string{"text"},
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
	if v.Consult.Status != "proposed" || v.Consult.Proposal == nil || v.Consult.Proposal.PriceYen != 9000 || v.Consult.Proposal.FeePercent != 20 || v.Consult.Proposal.CoachReceivesYen != 7200 || v.Consult.Proposal.DurationDays != 40 {
		t.Fatalf("proposal %+v", v.Consult)
	}
	// 他人は相談を読めない。
	_, tother := demoUser(t, h, bornYearsAgo(20))
	if r := doJSON(t, h, "GET", "/v1/support-consults/"+id, tother, nil); r.Code != 404 {
		t.Fatalf("other %d", r.Code)
	}
	// 選手が受けると、既存の契約（コーチの返事待ち）になり、中身は提案と一致する。
	r = doJSON(t, h, "POST", "/v1/support-consults/"+id+"/accept", ta, consentBody)
	if r.Code != 201 {
		t.Fatalf("accept %d %s", r.Code, r.Body.String())
	}
	ct := decodeContract(t, r.Body.Bytes())
	if ct.Status != "applied" {
		t.Fatalf("contract %+v", ct)
	}
	var price, days int
	var service, name string
	var compDate time.Time
	queryRowArgs(t, `SELECT price_yen, duration_days, service_type, competition_name, competition_date FROM coaching_contracts WHERE id=$1`, []any{ct.ID}, &price, &days, &service, &name, &compDate)
	if price != 9000 || days != 40 || service != "competition_support" || name != "秋季記録会" || compDate.Format("2006-01-02") != day(40) {
		t.Fatalf("snapshot %d %d %s %s %s", price, days, service, name, compDate)
	}
	// コーチが引き受けると、終了日は目標大会の日の終わりになる。
	if r := doJSON(t, h, "POST", "/v1/coach/contracts/"+ct.ID+"/accept", tc, consentBody); r.Code != 200 {
		t.Fatalf("coach accept %d %s", r.Code, r.Body.String())
	}
	var endsOK bool
	queryRowArgs(t, `SELECT ends_at IS NULL OR (ends_at AT TIME ZONE 'Asia/Tokyo')::date = ($2::date + 1) FROM coaching_contracts WHERE id=$1`, []any{ct.ID, day(40)}, &endsOK)
	if !endsOK {
		t.Fatal("ends_at is not the day after the competition")
	}
	// 契約が進行中のコーチへは、新しい無料相談を起こせない。
	if r := doJSON(t, h, "POST", "/v1/support-consults", ta, map[string]any{"coach_id": coach, "answers": consultAnswersBody()}); r.Code != 409 {
		t.Fatalf("consult during contract %d", r.Code)
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

func TestSupportConsultCalendarClosesReception(t *testing.T) {
	h := setup(t)
	coach, tc := consultCoach(t, h)
	_, ta := demoUser(t, h, bornYearsAgo(20))
	setDays := func(days ...map[string]any) {
		if r := doJSON(t, h, "PUT", "/v1/coach/me/availability", tc, map[string]any{"days": days}); r.Code != 200 {
			t.Fatalf("availability %d %s", r.Code, r.Body.String())
		}
	}
	// 開始希望日が閉じた日なら送れない。
	setDays(map[string]any{"date": day(5), "status": "unavailable"})
	a := consultAnswersBody()
	a["start_timing"] = "date"
	a["start_date"] = day(5)
	if r := doJSON(t, h, "POST", "/v1/support-consults", ta, map[string]any{"coach_id": coach, "answers": a}); r.Code != 422 {
		t.Fatalf("closed start date %d %s", r.Code, r.Body.String())
	}
	a["start_date"] = day(6)
	if r := doJSON(t, h, "POST", "/v1/support-consults", ta, map[string]any{"coach_id": coach, "answers": a}); r.Code != 201 {
		t.Fatalf("open start date %d %s", r.Code, r.Body.String())
	}
	// 今日が閉じていれば受付停止中（再開の目安つき）。送れない。
	_, ta2 := demoUser(t, h, bornYearsAgo(20))
	setDays(map[string]any{"date": day(0), "status": "unavailable"}, map[string]any{"date": day(1), "status": "unavailable"})
	r := doJSON(t, h, "GET", "/v1/coaches/"+coach.String()+"/support-consult", "", nil)
	if !strings.Contains(r.Body.String(), `"status":"paused"`) || !strings.Contains(r.Body.String(), `"resume_on":"`+day(2)+`"`) {
		t.Fatalf("paused %s", r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/support-consults", ta2, map[string]any{"coach_id": coach, "answers": consultAnswersBody()}); r.Code != 409 {
		t.Fatalf("paused send %d", r.Code)
	}
	// 閉じた期間が終われば、コーチの操作なしに受付中へ戻る。
	setDays(map[string]any{"date": day(0), "status": ""}, map[string]any{"date": day(1), "status": ""})
	r = doJSON(t, h, "GET", "/v1/coaches/"+coach.String()+"/support-consult", "", nil)
	if !strings.Contains(r.Body.String(), `"status":"open"`) {
		t.Fatalf("reopen %s", r.Body.String())
	}
}

func TestSupportConsultMinorGoesThroughGuardianFlow(t *testing.T) {
	h := setup(t)
	coach, tc := consultCoach(t, h)
	_, tm := demoUser(t, h, bornYearsAgo(15))
	id := sendConsult(t, h, tm, coach, consultAnswersBody())
	if v := decodeConsult(t, doJSON(t, h, "GET", "/v1/support-consults/"+id, tm, nil).Body.Bytes()); !v.Consult.Minor {
		t.Fatal("minor flag")
	}
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/propose", tc, proposalBody()); r.Code != 200 {
		t.Fatalf("propose %d %s", r.Code, r.Body.String())
	}
	r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/accept", tm, consentBody)
	if r.Code != 201 {
		t.Fatalf("accept %d %s", r.Code, r.Body.String())
	}
	ct := decodeContract(t, r.Body.Bytes())
	if !ct.Minor || ct.Status != "applied" || ct.StartsAt != nil {
		t.Fatalf("minor contract %+v", ct)
	}
	// コーチが引き受けても、保護者の同意とお支払いが済むまで始まらない。
	if r := doJSON(t, h, "POST", "/v1/coach/contracts/"+ct.ID+"/accept", tc, consentBody); r.Code != 200 {
		t.Fatalf("coach accept %d", r.Code)
	}
	if got := decodeContract(t, doJSON(t, h, "GET", "/v1/coaching/contracts/"+ct.ID, tm, nil).Body.Bytes()); got.Status != "accepted" || got.StartsAt != nil {
		t.Fatalf("minor after accept %+v", got)
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
	// 大会までプランを担当していなければ受け付けられない。
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
