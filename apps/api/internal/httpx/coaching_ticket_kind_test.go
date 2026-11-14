package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// 本体 #687：申込みで選んだチケットの種別（通常／無料）のロットだけを確保・消費する。

type ticketKindBalanceView struct {
	Available     int        `json:"available"`
	NextExpiresAt *time.Time `json:"next_expires_at"`
}

func ticketBalanceByKind(t *testing.T, h http.Handler, tok string) (normal, reward ticketKindBalanceView) {
	t.Helper()
	r := doJSON(t, h, http.MethodGet, "/v1/me/coaching/tickets", tok, nil)
	if r.Code != 200 {
		t.Fatalf("tickets %d %s", r.Code, r.Body.String())
	}
	var out struct {
		ByKind map[string]ticketKindBalanceView `json:"balance_by_kind"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	n, okN := out.ByKind["normal"]
	w, okW := out.ByKind["reward"]
	if !okN || !okW {
		t.Fatalf("balance_by_kind missing keys: %s", r.Body.String())
	}
	return n, w
}

func ticketKindReview(t *testing.T, h http.Handler, tok, kind string) (int, string, string) {
	t.Helper()
	body := reviewBodyFor(t, h, tok)
	body["method"] = "tickets"
	body["ticket_kind"] = kind
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tok, body)
	var out struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out.ID, rec.Body.String()
}

// addRewardLot は、紹介特典のロット（1枚）を、付与の台帳行つきで直接足す（紹介の数え方は #687 の範囲外なので経由しない）。
func addRewardLot(t *testing.T, owner uuid.UUID, expires time.Time) string {
	t.Helper()
	id := uuid.NewString()
	execSQLArgs(t, `INSERT INTO coaching_ticket_lots(id, owner_id, quantity, available, expires_at, kind) VALUES ($1,$2,1,1,$3,'referral_reward')`, id, owner, expires)
	execSQLArgs(t, `INSERT INTO coaching_ticket_ledger(id, lot_id, operation_key, kind, available_delta) VALUES (gen_random_uuid(),$1::uuid,'referral-grant-'||$1::text,'referral_grant',1)`, id)
	return id
}

func reservedLotKinds(t *testing.T, requestID string) string {
	t.Helper()
	var kinds string
	queryRowArgs(t, `SELECT string_agg(l.kind, ',' ORDER BY l.kind) FROM coaching_ticket_allocations a
		JOIN coaching_ticket_reservations r ON r.id=a.reservation_id JOIN coaching_ticket_lots l ON l.id=a.lot_id
		WHERE r.request_id=$1`, []any{requestID}, &kinds)
	return kinds
}

func TestTicketKindReservesOnlyChosenKind(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	ownerID, tok := demoRequester(t, h, admin)
	grantedLot := addRewardLot(t, ownerID, time.Now().Add(180*24*time.Hour))
	buySixTickets(t, h, tok)
	// 期限の早い特典ロットをもう1つ持たせる（複数あれば期限の早い方から使う）。
	early := time.Now().Add(90 * 24 * time.Hour).UTC().Truncate(time.Second)
	earlyLot := addRewardLot(t, ownerID, early)

	normal, reward := ticketBalanceByKind(t, h, tok)
	if normal.Available != 6 || reward.Available != 2 {
		t.Fatalf("by kind normal=%+v reward=%+v", normal, reward)
	}
	if reward.NextExpiresAt == nil || !reward.NextExpiresAt.Equal(early) {
		t.Fatalf("reward next expiry %v, want %v", reward.NextExpiresAt, early)
	}
	if normal.NextExpiresAt == nil || !normal.NextExpiresAt.After(early) {
		t.Fatalf("normal next expiry %v", normal.NextExpiresAt)
	}
	// 合計の残高（他画面・#684 の節が使う）は従来どおり。
	if got := ticketSummary(t, h, tok).Balance.Available; got != 8 {
		t.Fatalf("total balance %d", got)
	}

	// 無料を選ぶと特典ロットだけ。期限の早いものから。
	code, id, body := ticketKindReview(t, h, tok, "reward")
	if code != 201 {
		t.Fatalf("reward review %d %s", code, body)
	}
	var allocated string
	queryRowArgs(t, `SELECT a.lot_id::text FROM coaching_ticket_allocations a JOIN coaching_ticket_reservations r ON r.id=a.reservation_id WHERE r.request_id=$1`, []any{id}, &allocated)
	if allocated != earlyLot {
		t.Fatalf("reward used lot %s, want earliest %s (granted %s)", allocated, earlyLot, grantedLot)
	}
	normal, reward = ticketBalanceByKind(t, h, tok)
	if normal.Available != 6 || reward.Available != 1 {
		t.Fatalf("after reward normal=%+v reward=%+v", normal, reward)
	}

	// 通常を選ぶと通常ロットだけ。
	code, id, body = ticketKindReview(t, h, tok, "normal")
	if code != 201 {
		t.Fatalf("normal review %d %s", code, body)
	}
	if kinds := reservedLotKinds(t, id); kinds != "purchase" {
		t.Fatalf("normal used %q", kinds)
	}
	normal, reward = ticketBalanceByKind(t, h, tok)
	if normal.Available != 5 || reward.Available != 1 {
		t.Fatalf("after normal normal=%+v reward=%+v", normal, reward)
	}

	if code, _, body = ticketKindReview(t, h, tok, "reward"); code != 201 {
		t.Fatalf("second reward %d %s", code, body)
	}
	normal, reward = ticketBalanceByKind(t, h, tok)
	if normal.Available != 5 || reward.Available != 0 || reward.NextExpiresAt != nil {
		t.Fatalf("reward exhausted normal=%+v reward=%+v", normal, reward)
	}

	// 0枚の種別を直接指定してもエラー。通常チケットで代用しない。
	var before int
	queryRowArgs(t, `SELECT count(*) FROM review_requests WHERE requester_id=$1`, []any{ownerID}, &before)
	code, _, body = ticketKindReview(t, h, tok, "reward")
	if code != 422 || !strings.Contains(body, "ticket_kind_unavailable") {
		t.Fatalf("empty reward %d %s", code, body)
	}
	var after int
	queryRowArgs(t, `SELECT count(*) FROM review_requests WHERE requester_id=$1`, []any{ownerID}, &after)
	if after != before {
		t.Fatalf("failed reservation left request %d -> %d", before, after)
	}
	normal, reward = ticketBalanceByKind(t, h, tok)
	if normal.Available != 5 || reward.Available != 0 {
		t.Fatalf("substituted normal=%+v reward=%+v", normal, reward)
	}
	assertTicketAccounting(t, h, admin)
}

func TestTicketKindNormalUnavailableDoesNotUseReward(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	ownerID, tok := demoRequester(t, h, admin)
	addRewardLot(t, ownerID, time.Now().Add(180*24*time.Hour))
	code, _, body := ticketKindReview(t, h, tok, "normal")
	if code != 422 || !strings.Contains(body, "ticket_kind_unavailable") {
		t.Fatalf("empty normal %d %s", code, body)
	}
	if _, reward := ticketBalanceByKind(t, h, tok); reward.Available != 1 {
		t.Fatalf("reward consumed for normal %+v", reward)
	}
	// 種別の指定なしは従来どおり（使えるロットから自動）。
	body2 := reviewBodyFor(t, h, tok)
	body2["method"] = "tickets"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tok, body2); rec.Code != 201 {
		t.Fatalf("unspecified kind %d %s", rec.Code, rec.Body.String())
	}
	assertTicketAccounting(t, h, admin)
}

func TestTicketKindValidation(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	_, tok := demoRequester(t, h, admin)
	buySixTickets(t, h, tok)
	if code, _, body := ticketKindReview(t, h, tok, "gold"); code != 422 || !strings.Contains(body, "ticket_kind") {
		t.Fatalf("unknown kind %d %s", code, body)
	}
	// 単発（カード）にチケットの種別は付けられない。
	body := reviewBodyFor(t, h, tok)
	body["method"] = "card"
	body["ticket_kind"] = "normal"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tok, body); rec.Code != 422 || !strings.Contains(rec.Body.String(), "ticket_kind") {
		t.Fatalf("card with kind %d %s", rec.Code, rec.Body.String())
	}
	if got := ticketSummary(t, h, tok).Balance.Available; got != 6 {
		t.Fatalf("balance changed %d", got)
	}
	// 特典を持たない人のサマリでも両方のキーが出る。
	normal, reward := ticketBalanceByKind(t, h, tok)
	if normal.Available != 6 || reward.Available != 0 || reward.NextExpiresAt != nil {
		t.Fatalf("by kind normal=%+v reward=%+v", normal, reward)
	}
}
