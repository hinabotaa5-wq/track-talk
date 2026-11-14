package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/jackc/pgx/v5"
)

// setupIsolatedStripe は、共有の開発DBを使わず、テストごとの独立DBを作って
// その上で動くハンドラを返す。共有DBには競合する旧000167設計が残っているため、
// このファイルのテストでは setup/setupStripe を直接使わない。
func setupIsolatedStripe(t *testing.T) http.Handler {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://tracktalk:tracktalk@127.0.0.1:55432/tracktalk?sslmode=disable"
	}
	if err := config.RequireLocalDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	name := "coaching_referral_x_" + uuid.New().String()[:8]
	ident := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+ident); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("create isolated test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP DATABASE "+ident+" WITH (FORCE)"); err != nil {
			t.Errorf("drop test database: %v", err)
		}
		_ = admin.Close(ctx)
	})
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Database = name
	isolated, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	// setupConfig がマイグレーションする先を独立DBに向ける。
	t.Setenv("DATABASE_URL", "postgres://tracktalk:tracktalk@127.0.0.1:55432/"+name+"?sslmode=disable")
	_ = isolated.Close(ctx)
	h, _ := setupStripe(t)
	return h
}

// §12-5: 追加質問（kind=consultation）の購入は、有効な紹介コードが存在しても
// 正常に完了し、紹介カウント行を作らない。completeTicketCheckout が紹介を
// 数えるのは single/six だけであり、追加質問は別経路（completeConsultationCheckout）
// で完了して数えない。既存の追加質問テスト経路を使う。
func TestCoachingReferralConsultationPurchaseCreatesNoCount(t *testing.T) {
	h := setupIsolatedStripe(t)
	admin := adminToken(t, h)
	referrerID, referrer := demoRequester(t, h, admin)
	buySixTickets(t, h, referrer)
	var code string
	queryRowArgs(t, `SELECT code FROM coaching_referral_codes WHERE owner_id=$1`, []any{referrerID}, &code)
	if len(code) != 6 {
		t.Fatalf("referrer has no code: %q", code)
	}
	owner, _, root := endedCashRoom(t, h)
	// 買い手がその有効コードを提示しても有効と判定されることを確かめる。
	// 追加質問APIには紹介コード欄がなく、購入経路は kind=consultation で完結する。
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/referrals/check", owner, map[string]any{"code": code})
	var checked struct {
		Valid bool `json:"valid"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &checked) != nil || !checked.Valid {
		t.Fatalf("code check %d %s", rec.Code, rec.Body.String())
	}
	c := consultationDraft(t, h, owner, "/v1/coaching/contracts/"+root+"/consultations", uuid.NewString(), "契約終了後の追加質問")
	consultationAction(t, h, owner, c.ID, "submit", nil)
	consultationAction(t, h, owner, c.ID, "checkout", nil)
	if got := payConsultation(t, h, c.ID, 1200); got != 200 {
		t.Fatalf("consultation pay %d", got)
	}
	var state string
	queryRowArgs(t, `SELECT payment_state FROM coaching_consultations WHERE id=$1`, []any{c.ID}, &state)
	if state != "paid" {
		t.Fatalf("consultation payment_state = %q, want paid", state)
	}
	var counts int
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_counts`, []any{}, &counts)
	if counts != 0 {
		t.Fatalf("consultation purchase created %d referral counts", counts)
	}
}

// §12-13: 2人紹介の特典付与後、特典ロットを使い切って（reserve->consume）、
// 数えられていた購入の1件を返金しても、特典は取り消し・失効されず、
// 利用実績（予約・台帳）が残る（coaching_cancel_referral の consumed>0 枝）。
func TestCoachingReferralConsumedRewardSurvivesRefund(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupIsolatedStripe(t)
	admin := adminToken(t, h)
	referrerID, referrer := demoRequester(t, h, admin)
	buySixTickets(t, h, referrer)
	var code string
	queryRowArgs(t, `SELECT code FROM coaching_referral_codes WHERE owner_id=$1`, []any{referrerID}, &code)
	if len(code) != 6 {
		t.Fatalf("referrer has no code: %q", code)
	}
	checkoutWithCode := func(tok, referral string) string {
		t.Helper()
		rec := doJSON(t, h, http.MethodPost, "/v1/coaching/tickets/checkout", tok, map[string]any{
			"product_id": "six", "request_key": uuid.NewString(), "guardian_permission": true, "referral_code": referral,
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
	_, buyer1 := demoRequester(t, h, admin)
	_, buyer2 := demoRequester(t, h, admin)
	pack1 := checkoutWithCode(buyer1, code)
	checkoutWithCode(buyer2, code)
	var active int
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_counts WHERE referrer_id=$1 AND canceled_at IS NULL`, []any{referrerID}, &active)
	if active != 2 {
		t.Fatalf("friend count = %d, want 2", active)
	}
	var lotID string
	queryRowArgs(t, `SELECT lot_id FROM coaching_referral_rewards WHERE referrer_id=$1 AND canceled_at IS NULL`, []any{referrerID}, &lotID)
	if lotID == "" {
		t.Fatal("no reward granted")
	}
	// 6枚つづりを6回使い切ってから、7回目で特典ロットを予約・消費する。
	_, coach := onDutyCoach(t, h, admin)
	consumeReview := func() string {
		t.Helper()
		id := ticketReview(t, h, referrer)
		offers := coachRequests(t, h, coach).Reviewing
		if len(offers) != 1 {
			t.Fatalf("offers %+v", offers)
		}
		if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+offers[0].ID+"/start", coach, nil); rec.Code != 200 {
			t.Fatal(rec.Body.String())
		}
		if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", coach, responseBody()); rec.Code != 200 {
			t.Fatal(rec.Body.String())
		}
		return id
	}
	for i := 0; i < 6; i++ {
		consumeReview()
	}
	rewardReview := consumeReview()
	var available, reserved, consumed, expired int
	queryRowArgs(t, `SELECT available, reserved, consumed, expired FROM coaching_ticket_lots WHERE id=$1`, []any{lotID},
		&available, &reserved, &consumed, &expired)
	if available != 0 || reserved != 0 || consumed != 1 || expired != 0 {
		t.Fatalf("reward lot = %d/%d/%d/%d, want 0/0/1/0", available, reserved, consumed, expired)
	}
	// 数えられていた購入の1件を返金する（完了した返金の購入状態遷移）。
	execSQLArgs(t, `UPDATE coaching_ticket_purchases SET refunded_yen=amount_yen, status='refunded' WHERE id=$1`, pack1)
	queryRowArgs(t, `SELECT count(*) FROM coaching_referral_counts WHERE referrer_id=$1 AND canceled_at IS NULL`, []any{referrerID}, &active)
	if active != 1 {
		t.Fatalf("friend count after refund = %d, want 1", active)
	}
	var canceledAt *string
	var qualifyingRefundAt *string
	queryRowArgs(t, `SELECT canceled_at::text, qualifying_refund_at::text FROM coaching_referral_rewards WHERE lot_id=$1`, []any{lotID},
		&canceledAt, &qualifyingRefundAt)
	if canceledAt != nil {
		t.Fatalf("consumed reward was canceled at %s", *canceledAt)
	}
	if qualifyingRefundAt == nil {
		t.Fatal("qualifying refund was not recorded")
	}
	queryRowArgs(t, `SELECT available, reserved, consumed, expired FROM coaching_ticket_lots WHERE id=$1`, []any{lotID},
		&available, &reserved, &consumed, &expired)
	if available != 0 || reserved != 0 || consumed != 1 || expired != 0 {
		t.Fatalf("reward lot after refund = %d/%d/%d/%d, want 0/0/1/0", available, reserved, consumed, expired)
	}
	var consumeOps, expireOps int
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_ledger WHERE lot_id=$1 AND kind='consume'`, []any{lotID}, &consumeOps)
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_ledger WHERE lot_id=$1 AND kind='expire'`, []any{lotID}, &expireOps)
	if consumeOps != 1 || expireOps != 0 {
		t.Fatalf("reward ledger consume=%d expire=%d, want 1/0", consumeOps, expireOps)
	}
	var reservation string
	queryRowArgs(t, `SELECT status FROM coaching_ticket_reservations WHERE request_id=$1`, []any{rewardReview}, &reservation)
	if reservation != "consumed" {
		t.Fatalf("reward usage reservation = %q, want consumed", reservation)
	}
	var buyer1Count string
	queryRowArgs(t, `SELECT canceled_at::text FROM coaching_referral_counts WHERE referrer_id=$1 AND buyer_id=(SELECT owner_id FROM coaching_ticket_purchases WHERE id=$2)`, []any{referrerID, pack1}, &buyer1Count)
	if buyer1Count == "" {
		t.Fatal("refunded count row lost its cancellation mark")
	}
	assertTicketAccounting(t, h, admin)
}

// grantRewardLot は、購入なしの紹介者に2人ぶんの紹介購入で特典ロットを付与し、
// 紹介者のID・トークンと特典ロットIDを返す。
func grantRewardLot(t *testing.T, h http.Handler, admin string) (uuid.UUID, string, string) {
	t.Helper()
	referrerID, referrer := demoRequester(t, h, admin)
	// 紹介コードは通常は購入完了時に IssueCoachingReferral で発行される。
	// ここでは紹介者を購入なしのままにしたいため、同じ関数を直接呼ぶ。
	execSQLArgs(t, `SELECT coaching_issue_referral($1::uuid, 'purchase')`, referrerID)
	var code string
	queryRowArgs(t, `SELECT code FROM coaching_referral_codes WHERE owner_id=$1`, []any{referrerID}, &code)
	if len(code) != 6 {
		t.Fatalf("referrer has no code: %q", code)
	}
	for i := 0; i < 2; i++ {
		_, buyer := demoRequester(t, h, admin)
		rec := doJSON(t, h, http.MethodPost, "/v1/coaching/tickets/checkout", buyer, map[string]any{
			"product_id": "six", "request_key": uuid.NewString(), "guardian_permission": true, "referral_code": code,
		})
		var data struct {
			ID string `json:"purchase_id"`
		}
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &data) != nil || data.ID == "" {
			t.Fatalf("checkout %d %s", rec.Code, rec.Body.String())
		}
		payTicketPurchase(t, h, data.ID, 2500)
	}
	var lotID string
	queryRowArgs(t, `SELECT lot_id FROM coaching_referral_rewards WHERE referrer_id=$1 AND canceled_at IS NULL`, []any{referrerID}, &lotID)
	if lotID == "" {
		t.Fatal("no reward granted")
	}
	return referrerID, referrer, lotID
}

// consumeOneTicketReview は、利用者のチケット1枚で動画添削を1件やり切り、
// 依頼IDを返す（ticketReview→開始→回答の通常フロー）。
func consumeOneTicketReview(t *testing.T, h http.Handler, admin, userTok string) string {
	t.Helper()
	_, coach := onDutyCoach(t, h, admin)
	id := ticketReview(t, h, userTok)
	offers := coachRequests(t, h, coach).Reviewing
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+offers[0].ID+"/start", coach, nil); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", coach, responseBody()); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	return id
}

// §12-17: 購入なしの特典ロット（purchase_id NULL, kind='referral_reward'）は
// 残高1枚として数えられ、通常フロー（reserve->consume）で使える。台帳は
// grant相当（referral_grant）->reserve->consume の3行になる。コーチ報酬は
// 有料ロットと同じ通常の決済経路（RecordTicketReviewEarnings）で350円計上される。
// 既存テストの350円表明は有料のまとめ買い・単発・追加質問だけで、特典ロット
// 消費ぶんの表明はないため、ここで assert する。
func TestCoachingReferralRewardLotCountsAsBalanceAndSettles(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupIsolatedStripe(t)
	admin := adminToken(t, h)
	referrerID, referrer, lotID := grantRewardLot(t, h, admin)

	var kind string
	var purchaseRef *string
	var quantity, available, reserved, consumed, expired, refunded int
	queryRowArgs(t, `SELECT kind, purchase_id::text, quantity, available, reserved, consumed, expired, refunded FROM coaching_ticket_lots WHERE id=$1`,
		[]any{lotID}, &kind, &purchaseRef, &quantity, &available, &reserved, &consumed, &expired, &refunded)
	if kind != "referral_reward" || purchaseRef != nil {
		t.Fatalf("reward lot kind=%q purchase_ref=%v, want referral_reward/NULL", kind, purchaseRef)
	}
	if quantity != 1 || available != 1 || reserved != 0 || consumed != 0 || expired != 0 || refunded != 0 {
		t.Fatalf("reward lot = %d/%d/%d/%d/%d, want 1/0/0/0/0", available, reserved, consumed, expired, refunded)
	}
	var grantKind string
	var ga, gr, gc, gf, ge int
	var gpr *string
	var grantRows int
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_ledger WHERE lot_id=$1`, []any{lotID}, &grantRows)
	if grantRows != 1 {
		t.Fatalf("reward ledger rows = %d, want 1", grantRows)
	}
	queryRowArgs(t, `SELECT kind, available_delta, reserved_delta, consumed_delta, refunded_delta, expired_delta, purchase_ref::text FROM coaching_ticket_ledger WHERE lot_id=$1`,
		[]any{lotID}, &grantKind, &ga, &gr, &gc, &gf, &ge, &gpr)
	if grantKind != "referral_grant" || ga != 1 || gr != 0 || gc != 0 || gf != 0 || ge != 0 || gpr != nil {
		t.Fatalf("grant row kind=%q deltas=%d/%d/%d/%d/%d purchase_ref=%v", grantKind, ga, gr, gc, gf, ge, gpr)
	}
	// 購入なしでも残高1枚として数えられる（GET /v1/me/coaching/tickets = TicketBalance）。
	state := ticketSummary(t, h, referrer)
	if state.Balance.Available != 1 || state.Balance.Reserved != 0 {
		t.Fatalf("reward balance = %+v, want {1 0}", state.Balance)
	}
	if len(state.Purchases) != 0 {
		t.Fatalf("reward owner has %d purchases, want 0", len(state.Purchases))
	}

	reviewID := consumeOneTicketReview(t, h, admin, referrer)
	queryRowArgs(t, `SELECT available, reserved, consumed, expired FROM coaching_ticket_lots WHERE id=$1`, []any{lotID},
		&available, &reserved, &consumed, &expired)
	if available != 0 || reserved != 0 || consumed != 1 || expired != 0 {
		t.Fatalf("reward lot after use = %d/%d/%d/%d, want 0/0/1/0", available, reserved, consumed, expired)
	}
	var kindSeq string
	queryRowArgs(t, `SELECT string_agg(kind, ',' ORDER BY created_at, id) FROM coaching_ticket_ledger WHERE lot_id=$1`, []any{lotID}, &kindSeq)
	if kindSeq != "referral_grant,reserve,consume" {
		t.Fatalf("reward ledger kinds = %q, want referral_grant,reserve,consume", kindSeq)
	}
	var deltas string
	queryRowArgs(t, `SELECT string_agg(available_delta::text || '/' || reserved_delta::text || '/' || consumed_delta::text, ',' ORDER BY created_at, id) FROM coaching_ticket_ledger WHERE lot_id=$1`, []any{lotID}, &deltas)
	if deltas != "1/0/0,-1/1/0,0/-1/1" {
		t.Fatalf("reward ledger deltas = %q, want grant/reserve/consume steps", deltas)
	}
	var nullRefs int
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_ledger WHERE lot_id=$1 AND purchase_ref IS NULL`, []any{lotID}, &nullRefs)
	if nullRefs != 3 {
		t.Fatalf("reward ledger rows with purchase_ref NULL = %d, want 3", nullRefs)
	}
	var reservation string
	queryRowArgs(t, `SELECT status FROM coaching_ticket_reservations WHERE request_id=$1`, []any{reviewID}, &reservation)
	if reservation != "consumed" {
		t.Fatalf("reward usage reservation = %q, want consumed", reservation)
	}
	// 通常の決済経路でコーチに350円（単発500円の70%）が計上される。
	var gross, share, earnings int
	var source string
	queryRowArgs(t, `SELECT gross_yen, share_percent, amount_yen, source FROM coach_earnings WHERE review_request_id=$1`, []any{reviewID},
		&gross, &share, &earnings, &source)
	if source != "review" || gross != 500 || share != 70 || earnings != 350 {
		t.Fatalf("reward earning = %s %d/%d/%d, want review 500/70/350", source, gross, share, earnings)
	}
	var ownerCheck int
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_purchases WHERE owner_id=$1`, []any{referrerID}, &ownerCheck)
	if ownerCheck != 0 {
		t.Fatalf("reward owner made %d purchases, want 0", ownerCheck)
	}
	assertTicketAccounting(t, h, admin)
}

// §12-18: 特典ロットの付与・利用は、実在する集計パスから除外される。
//   - payment-counts（counts.py）: 支払い状態テーブルと orphan/row_counts だけを読み、
//     coaching_ticket_lots/ledger/reservations/rewards/earnings を読まない。ここでは
//     その入力テーブル群が特典フローで一切増減しないことと、orphan 相当が0件である
//     ことを表明する。counts.py プロセス自体の実行は psql を要するため UNVERIFIED。
//   - 照合クエリ（CoachingReconciliation + TicketReconciliation）: 管理API経由で表明。
//   - 管理集計（/v1/admin/coaching/tickets の購入一覧）: 特典は購入行を作らない。
//
// coach_earnings の350円は通常経路の計上であり、除外ではなく包含が正しい（§12-17）。
func TestCoachingReferralRewardExcludedFromAggregates(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupIsolatedStripe(t)
	admin := adminToken(t, h)
	referrerID, referrer, _ := grantRewardLot(t, h, admin)
	_ = consumeOneTicketReview(t, h, admin, referrer)

	// counts.py の STATUS_COLUMNS が読む支払いテーブル群: 特典フローで行は増えない。
	// 紹介で数えた2件の購入（paid の six）は買い手の正規の購入であり、特典由来ではない。
	for _, sql := range []string{
		`SELECT count(*) FROM review_payments`,
		`SELECT count(*) FROM coaching_payments`,
		`SELECT count(*) FROM coaching_consultations`,
		`SELECT count(*) FROM coaching_contracts`,
		`SELECT count(*) FROM captured_payment_refunds`,
		`SELECT count(*) FROM coaching_ticket_refunds`,
		`SELECT count(*) FROM coaching_consultation_refunds`,
		`SELECT count(*) FROM coach_payouts`,
		`SELECT count(*) FROM review_payment_earning_links`,
		`SELECT count(*) FROM captured_refund_earnings`,
		`SELECT count(*) FROM coach_refund_adjustments`,
		`SELECT count(*) FROM coach_refund_applications`,
		`SELECT count(*) FROM coach_refund_cancellations`,
		`SELECT count(*) FROM coach_refund_compensations`,
		`SELECT count(*) FROM coach_refund_application_reversals`,
	} {
		var got int
		queryRowArgs(t, sql, []any{}, &got)
		if got != 0 {
			t.Fatalf("%s = %d, want 0", sql, got)
		}
	}
	// 特典は購入行を作らない。購入テーブルにあるのは買い手2人の正規の six だけ。
	var purchases, paidSix int
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_purchases`, []any{}, &purchases)
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_purchases WHERE status='paid'`, []any{}, &paidSix)
	if purchases != 2 || paidSix != 2 {
		t.Fatalf("ticket purchases = %d (paid %d), want 2/2", purchases, paidSix)
	}
	var referrerPurchases int
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_purchases WHERE owner_id=$1`, []any{referrerID}, &referrerPurchases)
	if referrerPurchases != 0 {
		t.Fatalf("reward owner purchases = %d, want 0", referrerPurchases)
	}
	// counts.py の orphan 相当: 特典利用の earning は予約に紐づくため未リンク扱いにならない。
	var orphans int
	queryRowArgs(t, `SELECT count(*) FROM coach_earnings e WHERE e.source = 'review' AND NOT EXISTS (SELECT 1 FROM review_payment_earning_links l WHERE l.earning_id = e.id) AND NOT EXISTS (SELECT 1 FROM coaching_ticket_reservations t WHERE t.request_id = e.review_request_id)`, []any{}, &orphans)
	if orphans != 0 {
		t.Fatalf("review_earning_without_payment_link = %d, want 0", orphans)
	}
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_purchases p WHERE p.status IN ('paid','partially_refunded') AND p.owner_id IS NULL`, []any{}, &orphans)
	if orphans != 0 {
		t.Fatalf("ticket_purchase_paid_without_owner = %d, want 0", orphans)
	}
	queryRowArgs(t, `SELECT count(*) FROM review_payments p JOIN review_requests r ON r.id = p.request_id WHERE p.status = 'captured' AND r.returned_at IS NOT NULL AND r.coach_id IS NOT NULL AND NOT r.use_tickets AND NOT EXISTS (SELECT 1 FROM coach_earnings e WHERE e.review_request_id = r.id)`, []any{}, &orphans)
	if orphans != 0 {
		t.Fatalf("review_captured_returned_without_earning = %d, want 0", orphans)
	}

	// 照合クエリ: 特典ロットも台帳と一致し、購入なしロットが購入不足に数えられず、
	// 特典消費の350円が通常の earning として整合する。
	rec := doJSON(t, h, http.MethodGet, "/v1/admin/coaching/reconciliation", admin, nil)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var recon struct {
		TicketLotLedgerMismatches        int `json:"ticket_lot_ledger_mismatches"`
		PaidPurchasesWithoutLot          int `json:"paid_purchases_without_lot"`
		ConsumedTicketEarningsMismatches int `json:"consumed_ticket_earnings_mismatches"`
		PendingSupportRefunds            int `json:"pending_support_refunds"`
		PendingTicketRefunds             int `json:"pending_ticket_refunds"`
		PendingConsultationRefunds       int `json:"pending_consultation_refunds"`
		UnpaidCoachEarnings              int `json:"unpaid_coach_earnings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &recon); err != nil {
		t.Fatal(err)
	}
	if recon.TicketLotLedgerMismatches != 0 || recon.PaidPurchasesWithoutLot != 0 || recon.ConsumedTicketEarningsMismatches != 0 {
		t.Fatalf("reconciliation mismatches = %+v, want all zero", recon)
	}
	if recon.PendingSupportRefunds != 0 || recon.PendingTicketRefunds != 0 || recon.PendingConsultationRefunds != 0 {
		t.Fatalf("reconciliation pendings = %+v, want all zero", recon)
	}
	// 未送金の350円は特典利用の通常計上であり、除外ではなく包含が正しい（§12-17で表明）。
	if recon.UnpaidCoachEarnings != 1 {
		t.Fatalf("unpaid coach earnings = %d, want 1 (the reward 350 yen)", recon.UnpaidCoachEarnings)
	}

	// 管理集計: 購入一覧に特典は現れず、残高異常もない。
	adm := doJSON(t, h, http.MethodGet, "/v1/admin/coaching/tickets", admin, nil)
	if adm.Code != 200 {
		t.Fatal(adm.Body.String())
	}
	var ticketsOut struct {
		Purchases []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"purchases"`
		Issues []string `json:"balance_issues"`
	}
	if err := json.Unmarshal(adm.Body.Bytes(), &ticketsOut); err != nil {
		t.Fatal(err)
	}
	if len(ticketsOut.Purchases) != 2 {
		t.Fatalf("admin ticket purchases = %d, want 2 (paid six-packs only)", len(ticketsOut.Purchases))
	}
	if len(ticketsOut.Issues) != 0 {
		t.Fatalf("admin balance issues: %v", ticketsOut.Issues)
	}
	assertTicketAccounting(t, h, admin)
}
