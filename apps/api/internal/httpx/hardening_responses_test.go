package httpx_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// 応答項目の堅牢化：コーチ向けの追加質問と、チケット台帳の本人向け応答から、内部の項目を外す。

// コーチ向けの追加質問の応答には、支払う人・支払いの状態・重複防止のキー・未成年かどうかを載せない。
// 選手本人の応答は従来どおり。
func TestConsultationResponseForCoachOmitsInternalFields(t *testing.T) {
	h, _ := setupStripe(t)
	owner, coach, root := endedCashRoom(t, h)
	path := "/v1/coaching/contracts/" + root + "/consultations"
	m := marker()
	c := consultationDraft(t, h, owner, path, uuid.NewString(), m)
	consultationAction(t, h, owner, c.ID, "submit", nil)
	if code := payConsultation(t, h, c.ID, 1200); code != 200 {
		t.Fatalf("pay %d", code)
	}
	internal := []string{`"payer"`, `"payment_state"`, `"request_key"`, `"minor"`}
	coachBody := roomBody(t, h, coach, path)
	if !strings.Contains(coachBody, m) {
		t.Fatal("coach cannot see the consultation")
	}
	for _, k := range internal {
		if strings.Contains(coachBody, k) {
			t.Fatalf("coach room exposes %s: %s", k, coachBody)
		}
	}
	ownerBody := roomBody(t, h, owner, path)
	for _, k := range internal {
		if !strings.Contains(ownerBody, k) {
			t.Fatalf("owner room lost %s: %s", k, ownerBody)
		}
	}
	// コーチが操作した直後の応答にも載せない。
	r := doJSON(t, h, http.MethodPost, "/v1/coaching/consultations/"+c.ID+"/accept", coach, nil)
	if r.Code != 200 {
		t.Fatalf("accept %d %s", r.Code, r.Body.String())
	}
	for _, k := range internal {
		if strings.Contains(r.Body.String(), k) {
			t.Fatalf("coach action exposes %s: %s", k, r.Body.String())
		}
	}
}

// チケット台帳の本人向け応答には、台帳内部のキー（operation_key・lot_id）を載せない。残高・増減は従来どおり。
func TestTicketLedgerResponseOmitsInternalKeys(t *testing.T) {
	h, _ := setupStripe(t)
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	id := startTicketPurchase(t, h, tu, "six", uuid.NewString(), "")
	payTicketPurchase(t, h, id, 5000)
	r := doJSON(t, h, http.MethodGet, "/v1/me/coaching/tickets", tu, nil)
	if r.Code != 200 {
		t.Fatalf("tickets %d", r.Code)
	}
	body := r.Body.String()
	for _, k := range []string{`"operation_key"`, `"lot_id"`} {
		if strings.Contains(body, k) {
			t.Fatalf("ledger exposes %s: %s", k, body)
		}
	}
	if !strings.Contains(body, `"available_delta":6`) || ticketSummary(t, h, tu).Balance.Available != 6 {
		t.Fatalf("ledger changed: %s", body)
	}
}
