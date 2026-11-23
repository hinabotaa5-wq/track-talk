package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

func documentVersions() map[string]string {
	return map[string]string{"terms": "terms-2026-11-23", "guidelines": "guidelines-2026-11-15", "privacy": "privacy-2026-11-18"}
}

func TestDocumentConsentHistoryAndGate(t *testing.T) {
	h := setup(t)
	execSQLArgs(t, `DELETE FROM document_consents`)
	id := uuid.New()
	tok := token(t, id)
	check := func(want bool) {
		t.Helper()
		rec := doJSON(t, h, "GET", "/v1/me", tok, nil)
		var body struct {
			Documents *bool `json:"documents_consent"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != 200 || body.Documents == nil || *body.Documents != want {
			t.Fatalf("me: %d %s", rec.Code, rec.Body.String())
		}
	}
	check(false)
	if rec := doJSON(t, h, "POST", "/v1/me/consents/documents", "", map[string]any{"versions": documentVersions()}); rec.Code != 401 {
		t.Fatal(rec.Code)
	}
	for _, key := range []string{"terms", "guidelines", "privacy"} {
		versions := documentVersions()
		versions[key] = "old"
		if rec := doJSON(t, h, "POST", "/v1/me/consents/documents", tok, map[string]any{"versions": versions}); rec.Code != 409 {
			t.Fatalf("old %s: %d", key, rec.Code)
		}
		var count int
		queryRowArgs(t, `SELECT count(*) FROM document_consents WHERE user_id=$1`, []any{id}, &count)
		if count != 0 {
			t.Fatal("partial insert")
		}
	}
	for range 2 {
		if rec := doJSON(t, h, "POST", "/v1/me/consents/documents", tok, map[string]any{"versions": documentVersions()}); rec.Code != 204 {
			t.Fatalf("record %d %s", rec.Code, rec.Body.String())
		}
		check(true)
		// A retry must preserve this original timestamp, rather than refreshing it.
		execSQLArgs(t, `UPDATE document_consents SET consented_at='2026-01-01T00:00:00Z' WHERE user_id=$1`, id)
	}
	var count int
	queryRowArgs(t, `SELECT count(*) FROM document_consents WHERE user_id=$1 AND consented_at='2026-01-01T00:00:00Z'`, []any{id}, &count)
	if count != 3 {
		t.Fatalf("idempotent history: %d", count)
	}
	execSQLArgs(t, `DELETE FROM document_consents WHERE user_id=$1 AND document='privacy'`, id)
	check(false)
	execSQLArgs(t, `INSERT INTO document_consents(user_id,document,version) VALUES($1,'privacy','privacy-2000-01-01')`, id)
	check(false)
	// Historical versions stay present when a new version is accepted.
	if rec := doJSON(t, h, "POST", "/v1/me/consents/documents", tok, map[string]any{"versions": documentVersions()}); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	check(true)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	has, err := db.New(conn).HasDocumentConsents(ctx, db.HasDocumentConsentsParams{UserID: id, TermsVersion: "terms-next", GuidelinesVersion: documentVersions()["guidelines"], PrivacyVersion: documentVersions()["privacy"]})
	if err != nil || has {
		t.Fatalf("revised version should require consent: %v %v", has, err)
	}
}

func TestDocumentConsentsSurviveAccountDeletion(t *testing.T) {
	h := setup(t)
	execSQLArgs(t, `DELETE FROM document_consents`)
	id := uuid.New()
	tok := token(t, id)
	if rec := doJSON(t, h, "POST", "/v1/me/consents/documents", tok, map[string]any{"versions": documentVersions()}); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	paid := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	execSQLArgs(t, `INSERT INTO guardian_notices(id,minor_id,kind,guardian_email,purchased_at) VALUES($1,$2,'purchase','parent@example.com',$3)`, uuid.New(), id, paid)
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me", tok, map[string]string{"confirm": "退会する"}); rec.Code != 200 {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
	}
	var rows int
	queryRowArgs(t, `SELECT count(*) FROM document_consents WHERE user_id=$1 AND account_deleted_at IS NOT NULL AND last_transaction_at=$2`, []any{id, paid}, &rows)
	if rows != 3 {
		t.Fatalf("retained history: %d", rows)
	}
	queryRowArgs(t, `SELECT count(*) FROM profiles WHERE id=$1`, []any{id}, &rows)
	if rows != 0 {
		t.Fatal("profile remains")
	}
}

func TestDocumentConsentMigrationRoundTrip(t *testing.T) {
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
	for _, direction := range []string{"down", "up"} {
		src, err := os.ReadFile("../../migrations/000186_document_consents." + direction + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, string(src)); err != nil {
			t.Fatal(err)
		}
	}
	var rls bool
	var fks int
	if err = tx.QueryRow(ctx, `SELECT relrowsecurity FROM pg_class WHERE oid='document_consents'::regclass`).Scan(&rls); err != nil || !rls {
		t.Fatalf("RLS %v %v", rls, err)
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE conrelid='document_consents'::regclass AND contype='f'`).Scan(&fks); err != nil || fks != 0 {
		t.Fatalf("FKs %d %v", fks, err)
	}
	src, err := os.ReadFile("../../../../deploy/cloud-run/payment-counts/counts.py")
	if err != nil || !strings.Contains(string(src), "EXPECTED_MIGRATION = 189") {
		t.Fatalf("expected migration: %v", err)
	}
}
