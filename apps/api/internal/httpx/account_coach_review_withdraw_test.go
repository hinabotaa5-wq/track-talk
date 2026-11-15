package httpx_test

import (
	"net/http"
	"strings"
	"testing"
)

// #754: コーチとして対応中の1回プラン依頼があるあいだは退会できない。
// 新ブロッカー（WithdrawBlockers.open_review）は coach_id = 本人かつ
// status IN ('in_review', 'needs_info') を見る。支払い種別は問わない。

const coachReviewWithdrawMessage = "添削中の依頼があるあいだは退会できません。返却してから退会するか、お問い合わせからご連絡ください"

// withdrawCoachExpectConflict は、コーチ本人の退会が 409 review_in_progress で止まることを確かめる。
func withdrawCoachExpectConflict(t *testing.T, h http.Handler, tc string) {
	t.Helper()
	rec := doJSON(t, h, http.MethodDelete, "/v1/me", tc, map[string]string{"confirm": "退会する"})
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "review_in_progress") {
		t.Fatalf("withdraw while reviewing %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), coachReviewWithdrawMessage) {
		t.Fatalf("withdraw message %s", rec.Body.String())
	}
}

func assignedReviewState(t *testing.T, id string) (status, coachID string, unstarted bool) {
	t.Helper()
	queryRowArgs(t, `SELECT status, coach_id::text, started_at IS NULL FROM review_requests WHERE id=$1`, []any{id}, &status, &coachID, &unstarted)
	return status, coachID, unstarted
}

// 細目1: カード単発の対応中依頼（in_review、割当て済み・未返却）を担当するコーチは退会できない。
// 未着手（started_at が NULL）の割当て済みでも止まり、着手（start）後も止まる。
// 退会は進まない：アカウントが残り、コーチは approved のまま、依頼は in_review のまま coach_id も維持される。
func TestWithdrawBlockedByAssignedCardReview(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	id := chosenReview(t, h, tu, coach.String(), "card", false)
	payChosenReview(t, h, id, "card")

	if status, got, unstarted := assignedReviewState(t, id); status != "in_review" || got != coach.String() || !unstarted {
		t.Fatalf("assigned %s coach=%s unstarted=%v", status, got, unstarted)
	}
	// 未着手でも退会は止まる。
	withdrawCoachExpectConflict(t, h, tc)

	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/start", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	// 着手後も退会は止まる。
	withdrawCoachExpectConflict(t, h, tc)

	// 何も削除されていない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("me after blocked withdraw %d %s", rec.Code, rec.Body.String())
	}
	if ok, status, _ := coachProfileStatus(t, coach.String()); !ok || status != "approved" {
		t.Fatalf("coach %v %q (want approved)", ok, status)
	}
	if status, got, _ := assignedReviewState(t, id); status != "in_review" || got != coach.String() {
		t.Fatalf("request after blocked withdraw %s coach=%s", status, got)
	}
}

// 細目2: チケットで申し込まれた対応中依頼を担当するコーチも退会できない。
// 予約中のチケットは解放されない。
func TestWithdrawBlockedByAssignedTicketReview(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	buySixTickets(t, h, tu)
	id := ticketReview(t, h, tu)

	offers := coachRequests(t, h, tc).Reviewing
	if len(offers) != 1 || offers[0].ID != id {
		t.Fatalf("offers %+v", offers)
	}
	if status, got, _ := assignedReviewState(t, id); status != "in_review" || got != coach.String() {
		t.Fatalf("assigned %s coach=%s", status, got)
	}

	withdrawCoachExpectConflict(t, h, tc)

	state := ticketSummary(t, h, tu)
	if state.Balance.Available != 5 || state.Balance.Reserved != 1 {
		t.Fatalf("ticket released by blocked withdraw %+v", state.Balance)
	}
	if status, got, _ := assignedReviewState(t, id); status != "in_review" || got != coach.String() {
		t.Fatalf("request after blocked withdraw %s coach=%s", status, got)
	}
}

// 細目3: 差し戻し中（needs_info）の依頼を担当するコーチも退会できない。
func TestWithdrawBlockedByNeedsInfoReview(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	id := chosenReview(t, h, tu, coach.String(), "card", false)
	payChosenReview(t, h, id, "card")
	offers := coachRequests(t, h, tc).Reviewing
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/start", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	asked := askInfo(t, h, tc, id, map[string]any{"body": infoQuestion, "reason": "scene"}, http.StatusOK)
	if asked.Status != "needs_info" {
		t.Fatalf("asked %+v", asked)
	}

	withdrawCoachExpectConflict(t, h, tc)

	if got := reviewStatus(t, h, tu, id); got != "needs_info" {
		t.Fatalf("status after blocked withdraw %s", got)
	}
}

// 細目4: 対応中依頼を返却したあとは退会できる。
// 返却で記録された報酬は送金済みにしてから退会する（Stripe 設定ありのテスト環境では
// 未送金報酬のチェックが走るため。新ブロッカーが外れたことの確認が目的）。
func TestWithdrawAfterReviewReturn(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	id := chosenReview(t, h, tu, coach.String(), "card", false)
	payChosenReview(t, h, id, "card")
	offers := coachRequests(t, h, tc).Reviewing
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/start", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, responseBody()); rec.Code != http.StatusOK {
		t.Fatalf("response %d %s", rec.Code, rec.Body.String())
	}
	if got := reviewStatus(t, h, tu, id); got != "returned" {
		t.Fatalf("status %s", got)
	}
	// 返却は代金を確定させるが、報酬の記録は tick が行う。記録してから送金済みにしないと、
	// 退会時の recordCoachEarningsWith が拾った 350 円に未送金チェックが当たる。
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	markCoachEarningsPaid(t, coach.String())

	withdrawExpect(t, h, tc, http.StatusOK)

	if profileExists(t, coach.String()) {
		t.Fatal("profile remains after withdrawal")
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tc, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("me after withdraw %d %s", rec.Code, rec.Body.String())
	}
}

// 細目5: 対応中依頼が期限切れで expired になったあとは退会できる。
func TestWithdrawAfterReviewExpired(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	id := chosenReview(t, h, tu, coach.String(), "card", false)
	payChosenReview(t, h, id, "card")

	execSQLArgs(t, `UPDATE review_requests SET due_at=now()-interval '1 minute' WHERE id=$1`, id)
	execSQLArgs(t, `UPDATE review_payments SET authorized_at=now()-interval '2 minutes' WHERE request_id=$1`, id)
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	if got := reviewStatus(t, h, tu, id); got != "expired" {
		t.Fatalf("status %s", got)
	}

	withdrawExpect(t, h, tc, http.StatusOK)

	if profileExists(t, coach.String()) {
		t.Fatal("profile remains after withdrawal")
	}
}

// 細目6: 未割当ての指名（requested_coach_id がそのコーチで、まだ queued）だけでは退会は止まらない。
func TestWithdrawWithOnlyQueuedNomination(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	id := chosenReview(t, h, tu, coach.String(), "card", false)
	payChosenReview(t, h, id, "card")
	// 支払い済み・未割当ての指名の形は通常の申込フローだけでは作りにくいので、
	// TestReviewRandomAssignOff の流儀で直接更新して作る（requested_coach_id は残す）。
	execSQLArgs(t, `UPDATE review_requests SET status='queued', coach_id=NULL WHERE id=$1`, id)

	if got := reviewStatus(t, h, tu, id); got != "queued" {
		t.Fatalf("status %s (want queued)", got)
	}
	var coachNull bool
	var requested string
	queryRowArgs(t, `SELECT coach_id IS NULL, requested_coach_id::text FROM review_requests WHERE id=$1`, []any{id}, &coachNull, &requested)
	if !coachNull || requested != coach.String() {
		t.Fatalf("nomination coach_null=%v requested=%s", coachNull, requested)
	}

	withdrawExpect(t, h, tc, http.StatusOK)

	if profileExists(t, coach.String()) {
		t.Fatal("profile remains after withdrawal")
	}
	if got := reviewStatus(t, h, tu, id); got != "queued" {
		t.Fatalf("status after withdraw %s (want queued)", got)
	}
}
