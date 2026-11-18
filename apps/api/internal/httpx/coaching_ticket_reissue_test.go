package httpx_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// 本体 #797：運営者・コーチ都合で終了した特典チケットは、戻す時点で期限切れでも1枚を出し直す。

const reissueNoticeTitle = "紹介特典：期限切れの無料チケットを出し直しました"

// expiredRewardReview は、特典ロット1枚（申込み時は有効）で reward 種別のレビューを作り、
// その後にロットを期限切れにする（申込み→期限切れ→終了の順番を再現する）。
// 戻り値は依頼IDと使った特典ロットID。
func expiredRewardReview(t *testing.T, h http.Handler, owner uuid.UUID, tok, coachTok string) (requestID, lotID string) {
	t.Helper()
	lotID = addRewardLot(t, owner, time.Now().Add(180*24*time.Hour))
	code, id, body := ticketKindReview(t, h, tok, "reward")
	if code != http.StatusCreated {
		t.Fatalf("reward review %d %s", code, body)
	}
	if kinds := reservedLotKinds(t, id); kinds != "referral_reward" {
		t.Fatalf("reserved kinds = %q, want referral_reward", kinds)
	}
	execSQLArgs(t, `UPDATE coaching_ticket_lots SET expires_at = now() - interval '1 hour' WHERE id=$1`, lotID)
	return id, lotID
}

func rewardLotState(t *testing.T, lotID string) (available, reserved, consumed, expired int) {
	t.Helper()
	queryRowArgs(t, `SELECT available, reserved, consumed, expired FROM coaching_ticket_lots WHERE id=$1`, []any{lotID},
		&available, &reserved, &consumed, &expired)
	return available, reserved, consumed, expired
}

// reissuedRewardLot は、出し直しで増えた特典ロット（元のロット以外）を返す。なければ ""。
func reissuedRewardLot(t *testing.T, owner uuid.UUID, oldLot string) string {
	t.Helper()
	var count int
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_lots WHERE owner_id=$1 AND kind='referral_reward' AND id::text<>$2`,
		[]any{owner, oldLot}, &count)
	if count == 0 {
		return ""
	}
	var id string
	queryRowArgs(t, `SELECT id::text FROM coaching_ticket_lots WHERE owner_id=$1 AND kind='referral_reward' AND id::text<>$2 ORDER BY created_at DESC LIMIT 1`,
		[]any{owner, oldLot}, &id)
	return id
}

func countReissueNotices(t *testing.T, h http.Handler, tok string) int {
	t.Helper()
	notices := coachingNotifications(t, h, tok)
	n := 0
	for _, notice := range notices {
		if notice.Title == reissueNoticeTitle {
			n++
			if !strings.Contains(notice.Body, "出し直しました") {
				t.Fatalf("reissue notice body = %q", notice.Body)
			}
		}
	}
	return n
}

// 細目1：コーチの辞退で終了し、戻す時点で特典ロットが期限切れなら、出し直した日を
// 付与日とする特典チケット1枚が新しく使える状態になり、元のロットは戻らない。
// 台帳（reissue の操作キー・referral_grant 種別）と通知も残り、集計は壊れない。
func TestRewardTicketReissuedOnCoachWithdrawAfterExpiry(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	_, coachTok := onDutyCoach(t, h, admin)
	owner, tok := demoRequester(t, h, admin)
	id, lot := expiredRewardReview(t, h, owner, tok, coachTok)

	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/withdraw", coachTok,
		map[string]any{"reason_code": "video_unviewable"}); rec.Code != http.StatusOK {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
	}
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}

	if a, r, c, e := rewardLotState(t, lot); a != 0 || r != 0 || c != 0 || e != 1 {
		t.Fatalf("old lot = %d/%d/%d/%d, want 0/0/0/1", a, r, c, e)
	}
	var kinds string
	queryRowArgs(t, `SELECT string_agg(kind, ',' ORDER BY created_at, id) FROM coaching_ticket_ledger WHERE lot_id=$1`, []any{lot}, &kinds)
	if kinds != "referral_grant,reserve,expire" {
		t.Fatalf("old lot ledger = %q, want referral_grant,reserve,expire", kinds)
	}

	fresh := reissuedRewardLot(t, owner, lot)
	if fresh == "" {
		t.Fatal("no reissued lot")
	}
	if a, r, c, e := rewardLotState(t, fresh); a != 1 || r != 0 || c != 0 || e != 0 {
		t.Fatalf("reissued lot = %d/%d/%d/%d, want 1/0/0/0", a, r, c, e)
	}
	var freshExpiry time.Time
	queryRowArgs(t, `SELECT expires_at FROM coaching_ticket_lots WHERE id=$1`, []any{fresh}, &freshExpiry)
	if time.Until(freshExpiry) < 150*24*time.Hour {
		t.Fatalf("reissued lot expires at %v, want about 6 months later", freshExpiry)
	}
	var freshKinds string
	queryRowArgs(t, `SELECT string_agg(kind, ',' ORDER BY created_at, id) FROM coaching_ticket_ledger WHERE lot_id=$1`, []any{fresh}, &freshKinds)
	if freshKinds != "referral_grant" {
		t.Fatalf("reissued lot ledger = %q, want referral_grant", freshKinds)
	}
	var freshKey string
	queryRowArgs(t, `SELECT operation_key FROM coaching_ticket_ledger WHERE lot_id=$1`, []any{fresh}, &freshKey)
	if !strings.HasPrefix(freshKey, "reissue-") {
		t.Fatalf("reissued lot operation_key = %q, want reissue- prefix", freshKey)
	}

	if got := countReissueNotices(t, h, tok); got != 1 {
		t.Fatalf("reissue notices = %d, want 1", got)
	}

	// 元のロットは二重に使える状態に戻らない：残高は出し直した1枚だけ。
	normal, reward := ticketBalanceByKind(t, h, tok)
	if reward.Available != 1 {
		t.Fatalf("reward balance = %d, want 1", reward.Available)
	}
	if normal.Available != 0 {
		t.Fatalf("normal balance = %d, want 0", normal.Available)
	}
	// 紹介の付与回数制限に触れない：rewards 行は増やさない。
	var rewards int
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_rewards WHERE referrer_id=$1`, []any{owner}, &rewards)
	if rewards != 0 {
		t.Fatalf("rewards rows = %d, want 0", rewards)
	}
	// 売上・返金・払戻しの計算対象にならない：購入行を作らない。
	var purchases int
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_purchases WHERE owner_id=$1`, []any{owner}, &purchases)
	if purchases != 0 {
		t.Fatalf("purchases = %d, want 0", purchases)
	}
	assertTicketAccounting(t, h, admin)
}

// 細目1（タイムアウト経路）：返却期限を守らなかった終了でも、期限切れの特典は出し直す。
func TestRewardTicketReissuedOnTimeoutAfterExpiry(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	_, coachTok := onDutyCoach(t, h, admin)
	owner, tok := demoRequester(t, h, admin)
	id, lot := expiredRewardReview(t, h, owner, tok, coachTok)

	execSQLArgs(t, `UPDATE review_requests SET due_at = now() - interval '1 minute' WHERE id=$1`, id)
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	var status string
	var timedOut *string
	queryRowArgs(t, `SELECT status, timed_out_at::text FROM review_requests WHERE id=$1`, []any{id}, &status, &timedOut)
	if status != "expired" || timedOut == nil {
		t.Fatalf("status = %s timed_out = %v, want expired/set", status, timedOut)
	}

	if a, r, c, e := rewardLotState(t, lot); a != 0 || r != 0 || c != 0 || e != 1 {
		t.Fatalf("old lot = %d/%d/%d/%d, want 0/0/0/1", a, r, c, e)
	}
	fresh := reissuedRewardLot(t, owner, lot)
	if fresh == "" {
		t.Fatal("no reissued lot")
	}
	if a, _, _, _ := rewardLotState(t, fresh); a != 1 {
		t.Fatalf("reissued lot available = %d, want 1", a)
	}
	if got := countReissueNotices(t, h, tok); got != 1 {
		t.Fatalf("reissue notices = %d, want 1", got)
	}
	assertTicketAccounting(t, h, admin)
}

// 細目3：選手側の都合（開始前キャンセル）では、期限切れの特典は従来どおり失効し、出し直さない。
func TestRewardTicketNotReissuedOnPlayerCancel(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	_, coachTok := onDutyCoach(t, h, admin)
	owner, tok := demoRequester(t, h, admin)
	id, lot := expiredRewardReview(t, h, owner, tok, coachTok)

	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/cancel", tok, nil); rec.Code != http.StatusOK {
		t.Fatalf("cancel %d %s", rec.Code, rec.Body.String())
	}
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}

	if a, r, c, e := rewardLotState(t, lot); a != 0 || r != 0 || c != 0 || e != 1 {
		t.Fatalf("old lot = %d/%d/%d/%d, want 0/0/0/1", a, r, c, e)
	}
	if fresh := reissuedRewardLot(t, owner, lot); fresh != "" {
		t.Fatalf("unexpected reissued lot %s", fresh)
	}
	if got := countReissueNotices(t, h, tok); got != 0 {
		t.Fatalf("reissue notices = %d, want 0", got)
	}
	normal, reward := ticketBalanceByKind(t, h, tok)
	if reward.Available != 0 || normal.Available != 0 {
		t.Fatalf("balance = %+v/%+v, want 0/0", normal, reward)
	}
	assertTicketAccounting(t, h, admin)
}

// 細目2：期限内の戻しは従来どおり元のロットへ戻り、新規付与は起きない。
func TestRewardTicketNotReissuedWhenReturnedInTime(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	_, coachTok := onDutyCoach(t, h, admin)
	owner, tok := demoRequester(t, h, admin)

	lot := addRewardLot(t, owner, time.Now().Add(180*24*time.Hour))
	code, id, body := ticketKindReview(t, h, tok, "reward")
	if code != http.StatusCreated {
		t.Fatalf("reward review %d %s", code, body)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/withdraw", coachTok,
		map[string]any{"reason_code": "video_unviewable"}); rec.Code != http.StatusOK {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
	}
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}

	if a, r, c, e := rewardLotState(t, lot); a != 1 || r != 0 || c != 0 || e != 0 {
		t.Fatalf("lot = %d/%d/%d/%d, want 1/0/0/0", a, r, c, e)
	}
	if fresh := reissuedRewardLot(t, owner, lot); fresh != "" {
		t.Fatalf("unexpected reissued lot %s", fresh)
	}
	if got := countReissueNotices(t, h, tok); got != 0 {
		t.Fatalf("reissue notices = %d, want 0", got)
	}
	assertTicketAccounting(t, h, admin)
}

// 細目4：購入チケットの戻し・失効の挙動は従来どおり変わらない。
func TestPurchaseTicketUnchangedOnCoachWithdrawAfterExpiry(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	_, coachTok := onDutyCoach(t, h, admin)
	owner, tok := demoRequester(t, h, admin)
	buySixTickets(t, h, tok)
	code, id, body := ticketKindReview(t, h, tok, "normal")
	if code != http.StatusCreated {
		t.Fatalf("ticket review %d %s", code, body)
	}
	var lot string
	queryRowArgs(t, `SELECT lot_id FROM coaching_ticket_allocations a JOIN coaching_ticket_reservations r ON r.id=a.reservation_id WHERE r.request_id=$1`, []any{id}, &lot)
	execSQLArgs(t, `UPDATE coaching_ticket_lots SET expires_at = now() - interval '1 hour' WHERE id=$1`, lot)

	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/withdraw", coachTok,
		map[string]any{"reason_code": "video_unviewable"}); rec.Code != http.StatusOK {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
	}
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}

	var quantity int
	queryRowArgs(t, `SELECT quantity FROM coaching_ticket_lots WHERE id=$1`, []any{lot}, &quantity)
	if a, r, c, e := rewardLotState(t, lot); a != 0 || r != 0 || c != 0 || e != quantity {
		t.Fatalf("purchase lot = %d/%d/%d/%d, want 0/0/0/%d", a, r, c, e, quantity)
	}
	var rewardLots int
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_lots WHERE owner_id=$1 AND kind='referral_reward'`, []any{owner}, &rewardLots)
	if rewardLots != 0 {
		t.Fatalf("reward lots = %d, want 0", rewardLots)
	}
	if got := countReissueNotices(t, h, tok); got != 0 {
		t.Fatalf("reissue notices = %d, want 0", got)
	}
	assertTicketAccounting(t, h, admin)
}

// 細目6：同じ終了処理が再実行されても（tick の再試行）、出し直しは二重に起きない。
func TestRewardTicketReissueIsIdempotent(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	_, coachTok := onDutyCoach(t, h, admin)
	owner, tok := demoRequester(t, h, admin)
	id, lot := expiredRewardReview(t, h, owner, tok, coachTok)

	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/withdraw", coachTok,
		map[string]any{"reason_code": "video_unviewable"}); rec.Code != http.StatusOK {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
	}
	for i := 0; i < 2; i++ {
		if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
			t.Fatal(rec.Body.String())
		}
	}
	var lots int
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_lots WHERE owner_id=$1 AND kind='referral_reward'`, []any{owner}, &lots)
	if lots != 2 {
		t.Fatalf("reward lots = %d, want 2 (old + reissued)", lots)
	}
	if got := countReissueNotices(t, h, tok); got != 1 {
		t.Fatalf("reissue notices = %d, want 1", got)
	}
	assertTicketAccounting(t, h, admin)
	if fresh := reissuedRewardLot(t, owner, lot); fresh == "" {
		t.Fatal("no reissued lot")
	}
}

// 細目7：返金済みの紹介に紐づく特典ロット（取消し対象）では、出し直さない。
func TestRewardTicketNotReissuedForRefundedReward(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	_, coachTok := onDutyCoach(t, h, admin)
	owner, tok := demoRequester(t, h, admin)
	id, lot := expiredRewardReview(t, h, owner, tok, coachTok)

	execSQLArgs(t, `INSERT INTO coaching_referral_rewards(lot_id, referrer_id, qualifying_refund_at) VALUES ($1,$2,now())`, lot, owner)

	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/withdraw", coachTok,
		map[string]any{"reason_code": "video_unviewable"}); rec.Code != http.StatusOK {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
	}
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}

	if a, r, c, e := rewardLotState(t, lot); a != 0 || r != 0 || c != 0 || e != 1 {
		t.Fatalf("old lot = %d/%d/%d/%d, want 0/0/0/1", a, r, c, e)
	}
	if fresh := reissuedRewardLot(t, owner, lot); fresh != "" {
		t.Fatalf("unexpected reissued lot %s", fresh)
	}
	if got := countReissueNotices(t, h, tok); got != 0 {
		t.Fatalf("reissue notices = %d, want 0", got)
	}
	assertTicketAccounting(t, h, admin)
}
