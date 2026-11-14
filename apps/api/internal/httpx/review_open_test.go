package httpx_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// 動画添削の申込導線（本体 #569 フェーズB）のテスト。

// 支払いのある環境では、招待コードを入れていない人も、生まれた年月を登録すれば単発・チケットで申し込める。
func TestReviewOpenWithoutInviteCodeWhenPaymentsConfigured(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	id := uuid.New()
	tu := token(t, id)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tu, nil); rec.Code != http.StatusOK {
		t.Fatalf("me %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/me/coaching", tu, nil); !strings.Contains(rec.Body.String(), `"review_open":true`) || !strings.Contains(rec.Body.String(), `"demo":false`) {
		t.Fatalf("coaching me %s", rec.Body.String())
	}
	execSQLArgs(t, `UPDATE profiles SET birth_month=$2 WHERE id=$1`, id, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	// チケットも招待コードなしで買える（販売の開閉は COACHING_TICKET_SALES_ENABLED のまま）。
	startTicketPurchase(t, h, tu, "six", uuid.NewString(), "/coaching/new")
	body := reviewBodyFor(t, h, tu)
	body["method"] = "card"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body); rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), "checkout_url") {
		t.Fatalf("request without invite code %d %s", rec.Code, rec.Body.String())
	}
}

// 支払いのない検証用（Stripe の鍵がない環境）では、これまでどおり招待コードを入れた人だけ。
func TestReviewStillNeedsInviteCodeWithoutPayments(t *testing.T) {
	h := setup(t)
	id := uuid.New()
	tu := token(t, id)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tu, nil); rec.Code != http.StatusOK {
		t.Fatalf("me %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/me/coaching", tu, nil); rec.Code == http.StatusOK && !strings.Contains(rec.Body.String(), `"review_open":false`) {
		t.Fatalf("coaching me %s", rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, map[string]any{"tier": "coach", "event": "100m", "question": "見てください"}); rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
		t.Fatalf("request without invite code in demo %d %s", rec.Code, rec.Body.String())
	}
}

// 失効の30日前・7日前に1回ずつ知らせる。7日前を切ってから初めて見つけたロットには7日前の分だけ送る。
func TestTicketExpiryReminders(t *testing.T) {
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	id, tu := demoRequester(t, h, admin)
	first := buySixTickets(t, h, tu)
	second := buySixTickets(t, h, tu)
	count := func() int {
		var n int
		queryRowArgs(t, `SELECT count(*)::int FROM notifications WHERE recipient_id=$1 AND title LIKE 'チケットの有効期限まで%'`, []any{id}, &n)
		return n
	}
	// 期限まで遠いうちは送らない。
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"ticket_expiry_reminders":0`) {
		t.Fatalf("far %d %s", rec.Code, rec.Body.String())
	}
	execSQLArgs(t, `UPDATE coaching_ticket_lots SET expires_at=now()+interval '20 days' WHERE purchase_id=$1`, first)
	execSQLArgs(t, `UPDATE coaching_ticket_lots SET expires_at=now()+interval '3 days' WHERE purchase_id=$1`, second)
	if rec := tick(t, h, "tick-secret"); !strings.Contains(rec.Body.String(), `"ticket_expiry_reminders":2`) {
		t.Fatalf("first reminders %s", rec.Body.String())
	}
	var body string
	queryRowArgs(t, `SELECT body FROM notifications WHERE recipient_id=$1 AND title='チケットの有効期限まであと7日以内です'`, []any{id}, &body)
	// 文面は表題1行だけ。枚数と失効日時はチケットの画面で見せる（本体 #609）。
	if body != "" {
		t.Fatalf("7-day body %s", body)
	}
	var r30, r7 int
	queryRowArgs(t, `SELECT (reminded_30_at IS NOT NULL)::int, (reminded_7_at IS NOT NULL)::int FROM coaching_ticket_lots WHERE purchase_id=$1`, []any{second}, &r30, &r7)
	if r30 != 1 || r7 != 1 {
		t.Fatalf("7-day lot marks %d %d", r30, r7)
	}
	// 同じ段階の知らせは2回送らない。
	if rec := tick(t, h, "tick-secret"); !strings.Contains(rec.Body.String(), `"ticket_expiry_reminders":0`) {
		t.Fatalf("repeat %s", rec.Body.String())
	}
	// 30日前の知らせを送ったロットも、7日前を切ったら7日前の知らせを1回送る。
	execSQLArgs(t, `UPDATE coaching_ticket_lots SET expires_at=now()+interval '6 days' WHERE purchase_id=$1`, first)
	if rec := tick(t, h, "tick-secret"); !strings.Contains(rec.Body.String(), `"ticket_expiry_reminders":1`) {
		t.Fatalf("7-day after 30 %s", rec.Body.String())
	}
	// ベルの知らせは同じ見出しをまとめることがあるので、30日前と7日前の両方が届いていることだけを見る。
	if got := count(); got < 2 {
		t.Fatalf("reminders %d", got)
	}
	var thirty int
	queryRowArgs(t, `SELECT count(*)::int FROM notifications WHERE recipient_id=$1 AND title='チケットの有効期限まであと30日以内です'`, []any{id}, &thirty)
	if thirty == 0 {
		t.Fatal("30-day reminder missing")
	}
	// 使い切ったロットには送らない。
	execSQLArgs(t, `UPDATE coaching_ticket_lots SET available=0, consumed=consumed+available, expires_at=now()+interval '20 days', reminded_30_at=NULL, reminded_7_at=NULL WHERE purchase_id=$1`, first)
	if rec := tick(t, h, "tick-secret"); !strings.Contains(rec.Body.String(), `"ticket_expiry_reminders":0`) {
		t.Fatalf("used up %s", rec.Body.String())
	}
}
