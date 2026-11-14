package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 本体 #686：紹介リワードの周単位化の受け入れテスト。

type referralView struct {
	Code        string  `json:"code"`
	Active      bool    `json:"active"`
	FriendCount int     `json:"friend_count"`
	GrantedAt   *string `json:"granted_at"`
}

func myReferral(t *testing.T, h http.Handler, tok string) *referralView {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/me/coaching/referral", tok, nil)
	var out struct {
		Referral *referralView `json:"referral"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
		t.Fatalf("referral %d %s", rec.Code, rec.Body.String())
	}
	return out.Referral
}

func referralCheck(t *testing.T, h http.Handler, tok, code string) bool {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/referrals/check", tok, map[string]any{"code": code})
	var out struct {
		Valid bool `json:"valid"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
		t.Fatalf("check %d %s", rec.Code, rec.Body.String())
	}
	return out.Valid
}

// buySixWithReferral は、紹介IDを入れて6回パックを買い、支払いまで済ませて購入IDを返す。
func buySixWithReferral(t *testing.T, h http.Handler, tok, code string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/tickets/checkout", tok, map[string]any{
		"product_id": "six", "request_key": uuid.NewString(), "guardian_permission": true, "referral_code": code,
	})
	var data struct {
		ID string `json:"purchase_id"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &data) != nil || data.ID == "" {
		t.Fatalf("checkout %d %s", rec.Code, rec.Body.String())
	}
	payTicketPurchase(t, h, data.ID, 2500)
	return data.ID
}

func activeCodeCount(t *testing.T, owner uuid.UUID) int {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_codes WHERE owner_id=$1 AND closed_at IS NULL`, []any{owner}, &n)
	return n
}

// 受け入れ 1・2・4・8：達成後の対象購入で新しいIDが発行され、進捗は0から。過去の周で
// カウント済みの購入者は新しい周で数えられない（購入者側には使えないIDとして伝わる）。
// 有効な紹介IDで買った購入は起点にならず、数えられなかった購入は起点になる。
func TestCoachingReferralCyclesRepeat(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupIsolatedStripe(t)
	admin := adminToken(t, h)
	ownerID, owner := demoRequester(t, h, admin)
	f1ID, f1 := demoRequester(t, h, admin)
	f2ID, f2 := demoRequester(t, h, admin)

	buySixWithReferral(t, h, owner, "")
	first := myReferral(t, h, owner)
	if first == nil || !first.Active || first.FriendCount != 0 {
		t.Fatalf("first cycle %+v", first)
	}
	buySixWithReferral(t, h, f1, first.Code)
	if r := myReferral(t, h, owner); r.Code != first.Code || r.FriendCount != 1 {
		t.Fatalf("progress is not carried within the cycle: %+v", r)
	}
	buySixWithReferral(t, h, f2, first.Code)
	done := myReferral(t, h, owner)
	if done.Code != first.Code || done.Active || done.GrantedAt == nil || done.FriendCount != 2 {
		t.Fatalf("completed cycle %+v", done)
	}
	// 有効な紹介IDで買った友達は、その購入では自分のIDが発行されない。
	for _, id := range []uuid.UUID{f1ID, f2ID} {
		var n int
		queryRowArgs(t, `SELECT count(*) FROM coaching_referral_codes WHERE owner_id=$1`, []any{id}, &n)
		if n != 0 {
			t.Fatalf("counted purchase became an origin for %s", id)
		}
	}
	// 終了したIDは以後使えない。数えられなかった購入はその購入者の起点になる。
	f3ID, f3 := demoRequester(t, h, admin)
	if referralCheck(t, h, f3, first.Code) {
		t.Fatal("closed code is still usable")
	}
	buySixWithReferral(t, h, f3, first.Code)
	if activeCodeCount(t, f3ID) != 1 {
		t.Fatal("uncounted purchase did not issue a code")
	}

	// 達成済みの本人が新たに対象購入をすると、新しいIDで進捗0から始まる。
	buySixWithReferral(t, h, owner, "")
	second := myReferral(t, h, owner)
	if second.Code == first.Code || !second.Active || second.FriendCount != 0 || second.GrantedAt != nil {
		t.Fatalf("second cycle %+v (first %s)", second, first.Code)
	}
	// 未達成のうちは、本人がさらに買ってもIDと進捗を引き継ぐ。
	buySixWithReferral(t, h, owner, "")
	if r := myReferral(t, h, owner); r.Code != second.Code {
		t.Fatalf("active code was reissued: %+v", r)
	}

	// 過去の周でカウント済みの f1 は、新しい周では数えられない。
	if referralCheck(t, h, f1, second.Code) {
		t.Fatal("previously counted buyer can use the new code")
	}
	buySixWithReferral(t, h, f1, second.Code)
	if r := myReferral(t, h, owner); r.FriendCount != 0 {
		t.Fatalf("previously counted buyer was counted again: %+v", r)
	}
	// 数えられなかったので、f1 の初めての起点購入になる。
	if activeCodeCount(t, f1ID) != 1 {
		t.Fatal("uncounted purchase did not become f1's origin")
	}

	_, f4 := demoRequester(t, h, admin)
	_, f5 := demoRequester(t, h, admin)
	buySixWithReferral(t, h, f4, second.Code)
	buySixWithReferral(t, h, f5, second.Code)
	var rewards, distinct, total int
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_rewards WHERE referrer_id=$1 AND canceled_at IS NULL`, []any{ownerID}, &rewards)
	if rewards != 2 {
		t.Fatalf("rewards = %d, want 2", rewards)
	}
	queryRowArgs(t, `SELECT count(DISTINCT code), count(*) FROM coaching_referral_codes`, nil, &distinct, &total)
	if distinct != total {
		t.Fatalf("codes reused: %d distinct of %d", distinct, total)
	}
	if activeCodeCount(t, ownerID) != 0 {
		t.Fatal("completed second cycle is still active")
	}
}

// 受け入れ 3：返金で取り消されたカウントの購入者は、再びカウントされ得る。
func TestCoachingReferralRefundedBuyerCountsAgain(t *testing.T) {
	h := setupIsolatedStripe(t)
	admin := adminToken(t, h)
	_, owner := demoRequester(t, h, admin)
	_, friend := demoRequester(t, h, admin)
	buySixWithReferral(t, h, owner, "")
	code := myReferral(t, h, owner).Code
	pack := buySixWithReferral(t, h, friend, code)
	execSQLArgs(t, "UPDATE coaching_ticket_purchases SET refunded_yen=amount_yen, status='refunded' WHERE id=$1", pack)
	if r := myReferral(t, h, owner); r.FriendCount != 0 {
		t.Fatalf("refunded count still valid: %+v", r)
	}
	if !referralCheck(t, h, friend, code) {
		t.Fatal("refunded buyer cannot use the code again")
	}
	buySixWithReferral(t, h, friend, code)
	if r := myReferral(t, h, owner); r.FriendCount != 1 {
		t.Fatalf("refunded buyer was not counted again: %+v", r)
	}
}

// 受け入れ 7：チャージバック（charge.dispute.created）でも、返金と同じくカウントと
// 未使用の特典を取り消す。特典が取り消された周は開き直り、別の友達で再び達成できる。
func TestCoachingReferralDisputeCancelsCountAndReward(t *testing.T) {
	h := setupIsolatedStripe(t)
	admin := adminToken(t, h)
	ownerID, owner := demoRequester(t, h, admin)
	_, f1 := demoRequester(t, h, admin)
	_, f2 := demoRequester(t, h, admin)
	buySixWithReferral(t, h, owner, "")
	code := myReferral(t, h, owner).Code
	buySixWithReferral(t, h, f1, code)
	disputed := buySixWithReferral(t, h, f2, code)
	var lot uuid.UUID
	queryRowArgs(t, `SELECT lot_id FROM coaching_referral_rewards WHERE referrer_id=$1 AND canceled_at IS NULL`, []any{ownerID}, &lot)

	if rec := stripeWebhook(t, h, testWebhookSecret, "charge.dispute.created", map[string]any{
		"id": "dp_referral", "amount": 2500, "reason": "fraudulent", "payment_intent": "pi_ticket_" + disputed,
	}); rec.Code != 200 {
		t.Fatalf("dispute %d %s", rec.Code, rec.Body.String())
	}
	var available int
	var canceled bool
	queryRowArgs(t, `SELECT l.available, r.canceled_at IS NOT NULL FROM coaching_ticket_lots l
		JOIN coaching_referral_rewards r ON r.lot_id=l.id WHERE l.id=$1`, []any{lot}, &available, &canceled)
	if available != 0 || !canceled {
		t.Fatalf("disputed reward available=%d canceled=%v", available, canceled)
	}
	r := myReferral(t, h, owner)
	if r.Code != code || !r.Active || r.FriendCount != 1 {
		t.Fatalf("cycle after dispute %+v", r)
	}
	// 同じ知らせが再送されても何も変わらない。
	if rec := stripeWebhook(t, h, testWebhookSecret, "charge.dispute.created", map[string]any{
		"id": "dp_referral", "payment_intent": "pi_ticket_" + disputed,
	}); rec.Code != 200 {
		t.Fatalf("dispute retry %d", rec.Code)
	}
	_, f3 := demoRequester(t, h, admin)
	buySixWithReferral(t, h, f3, code)
	var active int
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_rewards WHERE referrer_id=$1 AND canceled_at IS NULL`, []any{ownerID}, &active)
	if active != 1 {
		t.Fatalf("reopened cycle was not granted again: %d", active)
	}
}

// 受け入れ 4（後半）：特典チケットの使用は対象購入ではなく、IDの発行・周の起点にならない。
func TestCoachingReferralRewardUseIsNotOrigin(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupIsolatedStripe(t)
	admin := adminToken(t, h)
	referrerID, referrer, lotID := grantRewardLot(t, h, admin)
	if activeCodeCount(t, referrerID) != 0 {
		t.Fatal("completed cycle is still active")
	}
	consumeOneTicketReview(t, h, admin, referrer)
	var consumed int
	queryRowArgs(t, `SELECT consumed FROM coaching_ticket_lots WHERE id=$1`, []any{lotID}, &consumed)
	if consumed != 1 {
		t.Fatalf("reward lot consumed=%d", consumed)
	}
	var codes int
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_codes WHERE owner_id=$1`, []any{referrerID}, &codes)
	if codes != 1 {
		t.Fatalf("using the reward ticket issued a code: %d codes", codes)
	}
}

// insertReviewPayment は、紹介の判定に必要な最小限の動画添削の支払い行を作る。
func insertReviewPayment(t *testing.T, payer uuid.UUID, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	captured := "NULL"
	if status == "captured" {
		captured = "now()"
	}
	execSQLArgs(t, `INSERT INTO review_payments(id,request_id,payer_id,method,amount_yen,checkout_session_id,checkout_url,session_expires_at,payment_intent_id,status,authorized_at,captured_at)
		VALUES ($1::uuid,NULL,$2,'card',500,'cs_'||$1::uuid::text,'https://example.test',now(),'pi_'||$1::uuid::text,$3,now(),`+captured+`)`, id, payer, status)
	return id
}

func profileID(t *testing.T, h http.Handler, admin string) uuid.UUID {
	t.Helper()
	id, _ := demoRequester(t, h, admin)
	return id
}

// 受け入れ 6：起点が未確定（単発カードの仮押さえ中）の間は、友達が2人そろっても付与を
// 保留し、確定した時点で付与する。仮押さえが解放されたらその周を無効にし、友達側の
// 重複排除は消費しない。現行の API はIDを支払い確定時に発行するためこの状態は通常
// 起きないが、付与条件として DB 側で担保していることを固定する。
func TestCoachingReferralOriginMustBeConfirmed(t *testing.T) {
	h := setupIsolatedStripe(t)
	admin := adminToken(t, h)

	// 確定で付与される周。
	owner := profileID(t, h, admin)
	origin := insertReviewPayment(t, owner, "authorized")
	execSQLArgs(t, `SELECT coaching_issue_referral_code($1, 'purchase', NULL, $2)`, owner, origin)
	var code string
	queryRowArgs(t, `SELECT code FROM coaching_referral_codes WHERE owner_id=$1`, []any{owner}, &code)
	for i := 0; i < 2; i++ {
		buyer := profileID(t, h, admin)
		execSQLArgs(t, `SELECT coaching_record_referral($1, $2, NULL, $3)`, buyer, code, insertReviewPayment(t, buyer, "captured"))
	}
	var rewards int
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_rewards WHERE referral_code=$1`, []any{code}, &rewards)
	if rewards != 0 {
		t.Fatal("granted before the origin was captured")
	}
	execSQLArgs(t, `UPDATE review_payments SET status='captured', captured_at=now() WHERE id=$1`, origin)
	var recipient *uuid.UUID
	queryRowArgs(t, `SELECT coaching_record_referral($1, NULL, NULL, $2)`, []any{owner, origin}, &recipient)
	if recipient == nil || *recipient != owner {
		t.Fatalf("held reward was not granted on capture: %v", recipient)
	}

	// 仮押さえが解放されて無効になる周。
	owner2 := profileID(t, h, admin)
	origin2 := insertReviewPayment(t, owner2, "authorized")
	execSQLArgs(t, `SELECT coaching_issue_referral_code($1, 'purchase', NULL, $2)`, owner2, origin2)
	var code2 string
	queryRowArgs(t, `SELECT code FROM coaching_referral_codes WHERE owner_id=$1`, []any{owner2}, &code2)
	friend := profileID(t, h, admin)
	execSQLArgs(t, `SELECT coaching_record_referral($1, $2, NULL, $3)`, friend, code2, insertReviewPayment(t, friend, "captured"))
	execSQLArgs(t, `UPDATE review_payments SET status='canceled', canceled_at=now() WHERE id=$1`, origin2)
	var voided bool
	var valid int
	queryRowArgs(t, `SELECT voided_at IS NOT NULL AND closed_at IS NOT NULL FROM coaching_referral_codes WHERE code=$1`, []any{code2}, &voided)
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_counts WHERE referral_code=$1 AND canceled_at IS NULL`, []any{code2}, &valid)
	if !voided || valid != 0 {
		t.Fatalf("released origin: voided=%v valid counts=%d", voided, valid)
	}
	// 友達は、本人の次の周で改めて数えられる。
	execSQLArgs(t, `SELECT coaching_record_referral($1, NULL, NULL, $2)`, owner2, insertReviewPayment(t, owner2, "captured"))
	var code3 string
	queryRowArgs(t, `SELECT code FROM coaching_referral_codes WHERE owner_id=$1 AND closed_at IS NULL`, []any{owner2}, &code3)
	execSQLArgs(t, `SELECT coaching_record_referral($1, $2, NULL, $3)`, friend, code3, insertReviewPayment(t, friend, "captured"))
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_counts WHERE referral_code=$1 AND canceled_at IS NULL`, []any{code3}, &valid)
	if valid != 1 {
		t.Fatalf("friend's dedupe was consumed by the voided cycle: %d", valid)
	}
}

// 受け入れ 9：同じ周の2人目のカウントが同時に処理されても、二重付与や進捗の不整合が起きない。
func TestCoachingReferralConcurrentSecondCount(t *testing.T) {
	h := setupIsolatedStripe(t)
	admin := adminToken(t, h)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for round := 0; round < 5; round++ {
		owner := profileID(t, h, admin)
		execSQLArgs(t, `SELECT coaching_record_referral($1, NULL, NULL, $2)`, owner, insertReviewPayment(t, owner, "captured"))
		var code string
		queryRowArgs(t, `SELECT code FROM coaching_referral_codes WHERE owner_id=$1`, []any{owner}, &code)
		first := profileID(t, h, admin)
		execSQLArgs(t, `SELECT coaching_record_referral($1, $2, NULL, $3)`, first, code, insertReviewPayment(t, first, "captured"))
		const racers = 4
		payments := make([][2]uuid.UUID, racers)
		for i := range payments {
			buyer := profileID(t, h, admin)
			payments[i] = [2]uuid.UUID{buyer, insertReviewPayment(t, buyer, "captured")}
		}
		var wg sync.WaitGroup
		errs := make(chan error, racers)
		start := make(chan struct{})
		for _, p := range payments {
			wg.Add(1)
			go func(buyer, payment uuid.UUID) {
				defer wg.Done()
				<-start
				_, err := pool.Exec(ctx, `SELECT coaching_record_referral($1, $2, NULL, $3)`, buyer, code, payment)
				errs <- err
			}(p[0], p[1])
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		var rewards, valid, lots int
		queryRowArgs(t, `SELECT count(*) FROM coaching_referral_rewards WHERE referral_code=$1`, []any{code}, &rewards)
		queryRowArgs(t, `SELECT count(*) FROM coaching_referral_counts WHERE referral_code=$1 AND canceled_at IS NULL`, []any{code}, &valid)
		queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_lots WHERE owner_id=$1 AND kind='referral_reward'`, []any{owner}, &lots)
		if rewards != 1 || valid != 2 || lots != 1 {
			t.Fatalf("round %d: rewards=%d valid counts=%d lots=%d", round, rewards, valid, lots)
		}
	}
}

// 受け入れ 6（API 経路）：単発カードは仮押さえ（決済完了）の時点で紹介IDを発行し、
// 友達2人がそろっても capture までは付与を保留し、capture で付与する。
// あわせて、紹介者にすでに数えられている購入者には check が already_counted を返す（受け入れ 2）。
func TestCoachingReferralCardOriginHeldUntilCapture(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupIsolatedStripe(t)
	admin := adminToken(t, h)
	_, coach := onDutyCoach(t, h, admin)
	payerID, payer := demoRequester(t, h, admin)

	body := reviewBodyFor(t, h, payer)
	body["method"] = "card"
	rec := createReviewWith(t, h, payer, "card", body)
	var review reviewRequestView
	if rec.Code != 201 || json.Unmarshal(rec.Body.Bytes(), &review) != nil || review.ID == "" {
		t.Fatalf("review checkout %d %s", rec.Code, rec.Body.String())
	}
	var session string
	queryRowArgs(t, "SELECT checkout_session_id FROM review_payments WHERE request_id=$1", []any{review.ID}, &session)
	if rec := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{
		"id": session, "payment_status": "unpaid", "payment_intent": "pi_card_origin",
		"metadata": map[string]string{"kind": "review"},
	}); rec.Code != 200 {
		t.Fatalf("authorize %d %s", rec.Code, rec.Body.String())
	}
	r := myReferral(t, h, payer)
	if r == nil || !r.Active {
		t.Fatalf("authorized card did not issue a code: %+v", r)
	}
	var friends []string
	for i := 0; i < 2; i++ {
		_, f := demoRequester(t, h, admin)
		buySixWithReferral(t, h, f, r.Code)
		friends = append(friends, f)
	}
	var rewards int
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_rewards WHERE referrer_id=$1`, []any{payerID}, &rewards)
	if rewards != 0 {
		t.Fatal("granted while the origin card was only authorized")
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+review.ID+"/start", coach, nil); rec.Code != 200 {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+review.ID+"/response", coach, responseBody()); rec.Code != 200 {
		t.Fatalf("return %d %s", rec.Code, rec.Body.String())
	}
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_rewards WHERE referrer_id=$1 AND canceled_at IS NULL`, []any{payerID}, &rewards)
	if rewards != 1 {
		t.Fatalf("held reward was not granted on capture: %d", rewards)
	}
	if done := myReferral(t, h, payer); done.Code != r.Code || done.Active || done.GrantedAt == nil {
		t.Fatalf("cycle after capture %+v", done)
	}
	check := doJSON(t, h, http.MethodPost, "/v1/coaching/referrals/check", friends[0], map[string]any{"code": r.Code})
	var out struct {
		Valid          bool `json:"valid"`
		AlreadyCounted bool `json:"already_counted"`
	}
	if check.Code != 200 || json.Unmarshal(check.Body.Bytes(), &out) != nil || out.Valid || !out.AlreadyCounted {
		t.Fatalf("check for counted buyer %d %s", check.Code, check.Body.String())
	}
}
