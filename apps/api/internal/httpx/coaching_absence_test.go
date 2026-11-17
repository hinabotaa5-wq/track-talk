package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

type absenceView struct {
	ID                string `json:"id"`
	Status            string `json:"status"`
	AbsentDays        int    `json:"absent_days"`
	ProposedEndsAt    string `json:"proposed_ends_at"`
	RefundEstimateYen int    `json:"refund_estimate_yen"`
	PriceYen          int    `json:"price_yen"`
}

func declareAbsence(t *testing.T, h http.Handler, tc, contract string, from, ret string) (int, absenceView, string) {
	t.Helper()
	r := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+contract+"/absences", tc, map[string]any{"absent_from": from, "return_on": ret, "reason": "体調不良のため"})
	var out struct {
		Absence absenceView `json:"absence"`
	}
	_ = json.Unmarshal(r.Body.Bytes(), &out)
	return r.Code, out.Absence, r.Body.String()
}

// paidConsultContract が置く終了日は日本時間の夕方になるので、本番の作り方（目標日の翌日の日本時間 0 時）に合わせ直す。
func alignSupportEnd(t *testing.T, contract string) {
	t.Helper()
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at=(competition_date+1)::timestamp AT TIME ZONE 'Asia/Tokyo' WHERE id=$1`, contract)
}

func contractEndsAt(t *testing.T, contract string) time.Time {
	t.Helper()
	var at time.Time
	queryRowArgs(t, `SELECT ends_at FROM coaching_contracts WHERE id=$1`, []any{contract}, &at)
	return at
}

func agreeAbsence(t *testing.T, h http.Handler, tok, path string, agree bool) (int, bool) {
	t.Helper()
	r := doJSON(t, h, http.MethodPost, path, tok, map[string]any{"agree": agree})
	var out struct {
		Agreed bool `json:"agreed"`
	}
	_ = json.Unmarshal(r.Body.Bytes(), &out)
	return r.Code, out.Agreed
}

// 申告→選手の同意→コーチの同意がそろったときだけ、終了日が不在日数分だけ後ろ倒しになる。
// 無返信・既読のみ・片方だけの同意では変わらない。履歴は別に残り、チャットが見られなくなった後も確認できる。
func TestAbsenceExtensionNeedsEveryonesExplicitConsent(t *testing.T) {
	h, _ := setupStripe(t)
	ta, tc, contract := paidConsultContract(t, h, 2, 30)
	alignSupportEnd(t, contract)
	original := contractEndsAt(t, contract)
	base := "/v1/coaching/contracts/" + contract + "/absences"

	// 選手は申告できない。不正な期間・理由も受け付けない。
	if code, _, body := declareAbsence(t, h, ta, contract, day(3), day(6)); code != http.StatusForbidden {
		t.Fatalf("player declare %d %s", code, body)
	}
	if code, _, _ := declareAbsence(t, h, tc, contract, day(3), ""); code != http.StatusUnprocessableEntity {
		t.Fatalf("no return date %d", code)
	}
	if r := doJSON(t, h, http.MethodPost, base, tc, map[string]any{"absent_from": day(3), "return_on": day(6), "reason": "連絡は @someone_123 まで"}); r.Code != http.StatusUnprocessableEntity {
		t.Fatalf("contact in reason %d", r.Code)
	}

	// 申告。延長案（新しい終了日）と終了案（返金概算額）が同じ提示に入っている。契約チャットに記録される。
	code, ab, body := declareAbsence(t, h, tc, contract, day(3), day(6))
	if code != http.StatusCreated || ab.Status != "declared" || ab.AbsentDays != 3 {
		t.Fatalf("declare %d %s", code, body)
	}
	if want := original.Add(72 * time.Hour); !strings.HasPrefix(ab.ProposedEndsAt, want.UTC().Format("2006-01-02T15:04")) {
		t.Fatalf("proposed %s want %v", ab.ProposedEndsAt, want)
	}
	if ab.RefundEstimateYen != 8100 || ab.PriceYen != 9000 { // 開始から3日目（今日）で終えた場合: 9,000円×27/30
		t.Fatalf("offer %+v", ab)
	}
	var found bool
	for _, m := range listMessages(t, h, ta, contract) {
		found = found || strings.Contains(m.Body, "【不在日の申告】")
	}
	if !found {
		t.Fatal("declaration not recorded in chat")
	}
	// 合意待ちの間に別の申告は出せない。
	if code, _, _ := declareAbsence(t, h, tc, contract, day(8), day(9)); code != http.StatusUnprocessableEntity {
		t.Fatalf("second open %d", code)
	}

	// 無返信・既読のみ・tick では変わらない。コーチだけ、選手だけの同意でも変わらない。
	if r := tick(t, h, "tick-secret"); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	_ = doJSON(t, h, http.MethodGet, base, ta, nil)
	_ = listMessages(t, h, ta, contract)
	if !contractEndsAt(t, contract).Equal(original) {
		t.Fatal("ends_at changed without consent")
	}
	if code, agreed := agreeAbsence(t, h, tc, base+"/"+ab.ID+"/coach-consent", true); code != http.StatusOK || agreed {
		t.Fatalf("coach only %d %v", code, agreed)
	}
	if !contractEndsAt(t, contract).Equal(original) {
		t.Fatal("ends_at changed with coach consent only")
	}
	// 他人は同意できない。コーチは選手の同意をできない。
	if code, _ := agreeAbsence(t, h, tc, base+"/"+ab.ID+"/consent", true); code != http.StatusForbidden {
		t.Fatalf("coach as player %d", code)
	}
	if code, _ := agreeAbsence(t, h, ta, base+"/"+ab.ID+"/coach-consent", true); code != http.StatusForbidden {
		t.Fatalf("player as coach %d", code)
	}
	if code, agreed := agreeAbsence(t, h, ta, base+"/"+ab.ID+"/consent", true); code != http.StatusOK || !agreed {
		t.Fatalf("player consent %d %v", code, agreed)
	}
	if got := contractEndsAt(t, contract); !got.Equal(original.Add(72 * time.Hour)) {
		t.Fatalf("ends_at %v want %v", got, original.Add(72*time.Hour))
	}
	var days int
	var comp, hist, price int
	queryRowArgs(t, `SELECT absence_days, price_yen FROM coaching_contracts WHERE id=$1`, []any{contract}, &days, &price)
	queryRowArgs(t, `SELECT count(*)::int FROM coaching_absence_extensions WHERE contract_id=$1 AND status='agreed' AND agreed_original_ends_at=$2 AND player_consented_by IS NOT NULL AND coach_consented_by IS NOT NULL AND coach_terms IS NOT NULL`, []any{contract, original}, &hist)
	// 分母（期間日数）は当初のまま、価格も変わらない。
	queryRowArgs(t, `SELECT (competition_date - (starts_at AT TIME ZONE 'Asia/Tokyo')::date + 1)::int FROM coaching_contracts WHERE id=$1`, []any{contract}, &comp)
	if days != 3 || hist != 1 || price != 9000 || comp != 30 {
		t.Fatalf("days %d history %d price %d denominator %d", days, hist, price, comp)
	}
	// 合意の記録が契約チャットにも残る。コーチへは変更後の取引条件が再明示される。
	var agreedMsg bool
	for _, m := range listMessages(t, h, tc, contract) {
		agreedMsg = agreedMsg || strings.Contains(m.Body, "【不在日の延長が成立しました】")
	}
	if !agreedMsg {
		t.Fatal("agreement not recorded in chat")
	}
	r := doJSON(t, h, http.MethodGet, base, tc, nil)
	if !strings.Contains(r.Body.String(), "取引条件の変更の明示") || !strings.Contains(r.Body.String(), "分母") {
		t.Fatalf("coach terms %s", r.Body.String())
	}
	// 2回目の不在は延長の対象外。
	if code, _, body := declareAbsence(t, h, tc, contract, day(10), day(11)); code != http.StatusUnprocessableEntity || !strings.Contains(body, "absence_second") {
		t.Fatalf("second absence %d %s", code, body)
	}
	// 終了して90日がたち、チャットが見られなくなった後も、合意内容は契約の画面から確認できる。
	execSQLArgs(t, `UPDATE coaching_contracts SET status='ended', ends_at=now()-interval '100 days' WHERE id=$1`, contract)
	r = doJSON(t, h, http.MethodGet, base, ta, nil)
	if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"status":"agreed"`) || !strings.Contains(r.Body.String(), "agreed_new_ends_at") {
		t.Fatalf("after chat closed %d %s", r.Code, r.Body.String())
	}
}

// 選手が延長しないと答えたら終了日は変わらず、あらためて別の申告ができる。上限と目標大会日の条件も API で効く。
func TestAbsenceDeclinedAndLimits(t *testing.T) {
	h, _ := setupStripe(t)
	ta, tc, contract := paidConsultContract(t, h, 2, 30)
	alignSupportEnd(t, contract)
	original := contractEndsAt(t, contract)
	base := "/v1/coaching/contracts/" + contract + "/absences"

	// 累計8日目以降は成立しない。
	if code, _, body := declareAbsence(t, h, tc, contract, day(3), day(11)); code != http.StatusUnprocessableEntity || !strings.Contains(body, "absence_limit") {
		t.Fatalf("8 days %d %s", code, body)
	}
	// 過去の日からは申告できない（事前の申告だけが対象）。
	if code, _, body := declareAbsence(t, h, tc, contract, day(-1), day(2)); code != http.StatusUnprocessableEntity || !strings.Contains(body, "absence_period_invalid") {
		t.Fatalf("past %d %s", code, body)
	}
	// 延長後の終了日が目標の大会日を過ぎる場合は提示されない。最終日は今日+27日。
	execSQLArgs(t, `UPDATE support_consults SET answers = jsonb_set(answers, '{competition_date}', to_jsonb($2::text)) WHERE contract_id=$1`, contract, day(28))
	if code, _, body := declareAbsence(t, h, tc, contract, day(3), day(6)); code != http.StatusUnprocessableEntity || !strings.Contains(body, "absence_past_goal") {
		t.Fatalf("past goal %d %s", code, body)
	}
	execSQLArgs(t, `UPDATE support_consults SET answers = jsonb_set(answers, '{competition_date}', to_jsonb($2::text)) WHERE contract_id=$1`, contract, day(30))
	code, ab, body := declareAbsence(t, h, tc, contract, day(3), day(6))
	if code != http.StatusCreated {
		t.Fatalf("declare %d %s", code, body)
	}
	if code, agreed := agreeAbsence(t, h, ta, base+"/"+ab.ID+"/consent", false); code != http.StatusOK || agreed {
		t.Fatalf("decline %d %v", code, agreed)
	}
	if !contractEndsAt(t, contract).Equal(original) {
		t.Fatal("ends_at changed after decline")
	}
	// 閉じた申告には、後から同意できない。
	if code, _ := agreeAbsence(t, h, tc, base+"/"+ab.ID+"/coach-consent", true); code != http.StatusUnprocessableEntity {
		t.Fatalf("consent after decline %d", code)
	}
	// 新しい申告ができ、その中身が返る。コーチは取り下げもできる。
	code, ab2, body := declareAbsence(t, h, tc, contract, day(5), day(7))
	if code != http.StatusCreated || ab2.AbsentDays != 2 {
		t.Fatalf("redeclare %d %s", code, body)
	}
	if code, _ := agreeAbsence(t, h, tc, base+"/"+ab2.ID+"/coach-consent", false); code != http.StatusOK {
		t.Fatalf("withdraw %d", code)
	}
	var n int
	queryRowArgs(t, `SELECT count(*)::int FROM coaching_absence_extensions WHERE contract_id=$1 AND status IN ('declined','withdrawn')`, []any{contract}, &n)
	if n != 2 || !contractEndsAt(t, contract).Equal(original) {
		t.Fatalf("history %d", n)
	}
}

// 未成年の契約では、保護者の同意がない限り終了日は変わらない。同意後に変わる。期間プランも対象。
func TestAbsenceExtensionMinorNeedsGuardian(t *testing.T) {
	h := setup(t)
	tm, gt := guardianMinor(t, h)
	tc, contract := minorAccepted(t, h, tm)
	execSQLArgs(t, `UPDATE coaching_contracts SET status='active', starts_at=now()-interval '1 day', ends_at=now()-interval '1 day'+make_interval(days=>duration_days) WHERE id=$1`, contract)
	original := contractEndsAt(t, contract)
	base := "/v1/coaching/contracts/" + contract + "/absences"

	code, ab, body := declareAbsence(t, h, tc, contract, day(2), day(4))
	if code != http.StatusCreated {
		t.Fatalf("declare %d %s", code, body)
	}
	if c, agreed := agreeAbsence(t, h, tm, base+"/"+ab.ID+"/consent", true); c != http.StatusOK || agreed {
		t.Fatalf("minor consent %d %v", c, agreed)
	}
	if c, agreed := agreeAbsence(t, h, tc, base+"/"+ab.ID+"/coach-consent", true); c != http.StatusOK || agreed {
		t.Fatalf("coach consent %d %v", c, agreed)
	}
	if !contractEndsAt(t, contract).Equal(original) {
		t.Fatal("ends_at changed without guardian consent")
	}
	// 保護者は自分のリンクから、契約の延長の提示を見て同意する。
	r := doJSON(t, h, http.MethodGet, "/v1/guardian/"+gt+"/contracts/"+contract+"/absences", "", nil)
	if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), ab.ID) {
		t.Fatalf("guardian view %d %s", r.Code, r.Body.String())
	}
	if c, agreed := agreeAbsence(t, h, "", "/v1/guardian/"+gt+"/contracts/"+contract+"/absences/"+ab.ID+"/consent", true); c != http.StatusOK || !agreed {
		t.Fatalf("guardian consent %d %v", c, agreed)
	}
	if got := contractEndsAt(t, contract); !got.Equal(original.Add(48 * time.Hour)) {
		t.Fatalf("ends_at %v want %v", got, original.Add(48*time.Hour))
	}
}

// 合意した不在日は精算で経過日数に数えない。30日間・不在3日・暦日25日目に途中終了 → 返金は 9,000円÷30×8＝2,400円、
// コーチ報酬は経過22日分（返金後の6,600円）の75%＝4,950円。
func TestAbsenceSupportSettlementExcludesAbsentDays(t *testing.T) {
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, _, contract := paidConsultContract(t, h, 24, 30)
	execSQLArgs(t, `UPDATE coaching_contracts SET
		absence_from=(starts_at AT TIME ZONE 'Asia/Tokyo')::date+10, absence_return_on=(starts_at AT TIME ZONE 'Asia/Tokyo')::date+13, absence_days=3,
		ends_at=ends_at+interval '3 days' WHERE id=$1`, contract)
	supportEnd(t, h, admin, contract, map[string]any{"cause": "player"})
	if len(f.refundList()) != 1 || f.refundList()[0].Get("amount") != "2400" {
		t.Fatalf("refund %+v", f.refundList())
	}
	if r := tick(t, h, "tick-secret"); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	var amount int
	queryRowArgs(t, `SELECT amount_yen FROM coach_earnings WHERE contract_id=$1`, []any{contract}, &amount)
	if amount != 4950 {
		t.Fatalf("final earning %d", amount)
	}
}

// 月次の仮払いは、延長を理由に留保・後ろ倒しされない（前の月の末日までの経過日数ぶん、不在日を除いて同じ月に記録される）。
func TestAbsenceDoesNotWithholdMonthlyAdvance(t *testing.T) {
	h, _ := setupStripe(t)
	_, _, contract := paidConsultContract(t, h, 0, 30)
	// 前の月の末日までに10日経過（開始は月初の10日前）。うち3日（開始から5〜7日目）が合意した不在日。
	execSQLArgs(t, `UPDATE coaching_contracts SET
		starts_at=(date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 10 + time '09:00') AT TIME ZONE 'Asia/Tokyo',
		competition_date=date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 10 + 29,
		absence_from=date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 6, absence_return_on=date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 3, absence_days=3,
		ends_at=((date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 10 + 30 + 3)::timestamp AT TIME ZONE 'Asia/Tokyo') WHERE id=$1`, contract)
	execSQLArgs(t, `INSERT INTO coaching_messages(id,contract_id,sender_role,body) VALUES(gen_random_uuid(),$1,'coach','よろしくお願いします')`, contract)
	if r := tick(t, h, "tick-secret"); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	var advance int
	queryRowArgs(t, `SELECT COALESCE(sum(amount_yen),0)::int FROM support_coach_advances WHERE contract_id=$1`, []any{contract}, &advance)
	// 経過10日−不在3日＝7日。未提供 9,000×23/30＝6,900円、経過分2,100円の75%＝1,575円。
	if advance != 1575 {
		t.Fatalf("advance %d", advance)
	}
}

// 不在の申告のチャット記録は、コーチの返信とは数えない（返信がない契約の自動返金の判定に使われる）。
func TestAbsenceMessageIsNotACoachReply(t *testing.T) {
	h, _ := setupStripe(t)
	_, tc, contract := paidConsultContract(t, h, 0, 30)
	execSQLArgs(t, `UPDATE coaching_contracts SET starts_at=now()-interval '80 hours' WHERE id=$1`, contract)
	if code, _, body := declareAbsence(t, h, tc, contract, day(3), day(5)); code != http.StatusCreated {
		t.Fatalf("declare %d %s", code, body)
	}
	if r := tick(t, h, "tick-secret"); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	var state string
	queryRowArgs(t, `SELECT fulfillment_state FROM coaching_contracts WHERE id=$1`, []any{contract}, &state)
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_contracts WHERE id=$1`, []any{contract}, &status)
	if status == "active" && state == "open" {
		t.Fatal("contract without a real coach reply was not auto-refunded")
	}
}
