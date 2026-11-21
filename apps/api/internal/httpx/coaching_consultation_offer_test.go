package httpx_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

func TestCoachingTranslationBackfillDeliveredConsultations(t *testing.T) {
	ai := &fakeTranslator{}
	h := setupTranslation(t, ai, nil)
	admin := adminToken(t, h)
	coachID, _ := demoRequester(t, h, admin)
	setLocale(t, coachID, "en")

	cases := []struct {
		status string
		marked bool
		want   bool
	}{
		{"draft", false, false},
		{"awaiting_payment", false, false},
		{"expired", false, false},
		{"cancelled", false, false},
		{"offered", true, true},
		{"in_review", true, true},
		{"answered", true, true},
		{"declined", true, true},
		{"expired", true, true},
		{"cancelled", true, true},
	}
	wantCalls := 0
	expectedTranslations := make(map[uuid.UUID]bool)
	for _, funding := range []string{"cash", "ticket"} {
		for _, tc := range cases {
			id := uuid.New()
			execSQLArgs(t, `INSERT INTO coaching_consultations
				(id, coach_id, request_key, review_request_id, minor, funding, price_yen, share_percent,
				 status, question, respond_by, answered_at, offered_at)
				VALUES ($1,$2,$1,$1,false,$3,500,70,$4,'大会前の調整について',
				        now(), CASE WHEN $4='answered' THEN now() END, CASE WHEN $5 THEN now() END)`,
				id, coachID, funding, tc.status, tc.marked)
			expectedTranslations[id] = tc.want
			if tc.want {
				wantCalls++
			}
		}
	}
	path := "/v1/admin/coaching-translations/backfill"
	assertCounts := func(translated, skipped int) {
		t.Helper()
		rec := doJSON(t, h, http.MethodPost, path, admin, nil)
		var counts struct{ Translated, Skipped, Failed int }
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &counts) != nil ||
			counts.Translated != translated || counts.Skipped != skipped || counts.Failed != 0 {
			t.Fatalf("backfill %d %s, want translated=%d skipped=%d failed=0", rec.Code, rec.Body.String(), translated, skipped)
		}
		if ai.calls() != wantCalls {
			t.Fatalf("translator calls=%d, want %d", ai.calls(), wantCalls)
		}
	}
	assertCounts(wantCalls, 0)
	assertCounts(0, wantCalls)
	for id, want := range expectedTranslations {
		var got bool
		queryRowArgs(t, `SELECT EXISTS (SELECT 1 FROM consultation_translations WHERE consultation_id=$1)`, []any{id}, &got)
		if got != want {
			t.Errorf("translation for consultation %s exists=%t, want %t", id, got, want)
		}
	}
}

func TestConsultationOfferMigrationAndTicketSubmission(t *testing.T) {
	_ = setup(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	migration := func(direction string) string {
		t.Helper()
		source, err := os.ReadFile("../../migrations/000188_consultation_offered_at." + direction + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		return string(source)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(migration("down"))
	insert := func(status string, respondBy, paidAt *time.Time) uuid.UUID {
		t.Helper()
		id := uuid.New()
		exec(`INSERT INTO coaching_consultations
			(id, request_key, review_request_id, minor, funding, price_yen, share_percent, status, respond_by, paid_at, answered_at)
			VALUES ($1,$1,$1,false,'ticket',500,70,$2,$3,$4, CASE WHEN $2='answered' THEN now() END)`,
			id, status, respondBy, paidAt)
		return id
	}
	respondBy := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	paidAt := respondBy.Add(-24 * time.Hour)
	fixtures := []struct {
		status string
		id     uuid.UUID
		want   *time.Time
	}{
		{"offered", insert("offered", &respondBy, &paidAt), &respondBy},
		{"in_review", insert("in_review", nil, &paidAt), &paidAt},
		{"answered", insert("answered", &respondBy, nil), &respondBy},
		{"declined", insert("declined", &respondBy, nil), &respondBy},
		{"expired", insert("expired", &respondBy, &paidAt), nil},
		{"cancelled", insert("cancelled", &respondBy, nil), nil},
		{"draft", insert("draft", nil, nil), nil},
		{"awaiting_payment", insert("awaiting_payment", nil, nil), nil},
	}
	missing := insert("offered", nil, nil)
	exec("SAVEPOINT missing_delivery_marker")
	if _, err := tx.Exec(ctx, migration("up")); err == nil || !strings.Contains(err.Error(), "no respond_by or paid_at") {
		t.Fatalf("migration should reject a delivered legacy row lacking both timestamps: %v", err)
	}
	exec("ROLLBACK TO SAVEPOINT missing_delivery_marker")
	exec("DELETE FROM coaching_consultations WHERE id=$1", missing)
	exec(migration("up"))
	for _, fixture := range fixtures {
		var got *time.Time
		if err := tx.QueryRow(ctx, "SELECT offered_at FROM coaching_consultations WHERE id=$1", fixture.id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if (got == nil) != (fixture.want == nil) || got != nil && !got.Equal(*fixture.want) {
			t.Fatalf("%s offered_at=%v, want %v", fixture.status, got, fixture.want)
		}
	}
	ticketDraft := fixtures[6].id
	q := db.New(tx)
	saved, err := q.SubmitTicketConsultation(ctx, ticketDraft)
	if err != nil || saved.Status != "offered" || saved.OfferedAt == nil {
		t.Fatalf("ticket submission status=%s offered_at=%v error=%v", saved.Status, saved.OfferedAt, err)
	}
	if _, err := q.SubmitTicketConsultation(ctx, ticketDraft); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("repeat ticket submission: %v", err)
	}
	closed, err := q.CloseConsultation(ctx, db.CloseConsultationParams{ID: ticketDraft, Status: "cancelled"})
	if err != nil || closed.OfferedAt == nil || !closed.OfferedAt.Equal(*saved.OfferedAt) {
		t.Fatalf("ticket cancellation changed delivery status: %v, %v", closed.OfferedAt, err)
	}
}
