package httpx_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 年齢未登録・成人・未成年の全員に同じ検証と取引ごとの記録を求める。
func TestPurchaseNoticeForEveryAccount(t *testing.T) {
	for _, age := range []string{"unregistered", "adult", "minor"} {
		t.Run(age, func(t *testing.T) {
			fakeCoachingCloudinary(t)
			h, f := setupCoachChoice(t)
			admin := adminToken(t, h)
			coach, _ := onDutyCoach(t, h, admin)
			birth := time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC)
			if age == "minor" {
				birth = time.Date(time.Now().Year()-14, 1, 1, 0, 0, 0, 0, time.UTC)
			}
			account, tok := demoUser(t, h, birth)
			if age == "unregistered" {
				execSQLArgs(t, `UPDATE profiles SET birth_month=NULL WHERE id=$1`, account)
			}
			for _, kind := range []string{"review", "ticket_purchase"} {
				path := "/v1/coaching/requests"
				body := reviewBodyFor(t, h, tok)
				body["method"] = "card"
				body["requested_coach_id"] = coach.String()
				if kind == "ticket_purchase" {
					path = "/v1/coaching/tickets/checkout"
					body = map[string]any{"product_id": "six", "request_key": uuid.NewString()}
				}
				delete(body, "guardian_permission")
				sessions := sessionCount(f)
				for _, checked := range []bool{false, true} {
					if checked {
						body["guardian_permission"] = false
					}
					rec := doJSON(t, h, http.MethodPost, path, tok, body)
					if rec.Code != 422 || sessionCount(f) != sessions {
						t.Fatalf("%s unchecked: %d %s", kind, rec.Code, rec.Body.String())
					}
				}
				body["guardian_permission"] = true
				body["guardian_permission_checked_at"] = time.Now().UTC().Format(time.RFC3339)
				rec := doJSON(t, h, http.MethodPost, path, tok, body)
				if rec.Code != 200 && rec.Code != 201 {
					t.Fatalf("%s checked: %d %s", kind, rec.Code, rec.Body.String())
				}
				var count int
				queryRowArgs(t, `SELECT count(*) FROM minor_purchase_declarations WHERE minor_id=$1 AND ref_type=$2 AND text_version='purchase-notice-2026-11-13' AND ref_id IS NOT NULL AND declared_at IS NOT NULL AND client_checked_at IS NOT NULL AND declared_birth_month IS NULL`, []any{account, kind}, &count)
				if count != 1 {
					t.Fatalf("%s evidence count=%d", kind, count)
				}
			}
		})
	}
}

// 既存の23歳ルールを維持し、新規の記録には生年月を写さず最後のチケット取引から5年を適用する。
func TestPurchaseNoticeRetentionPreservesLegacyRecords(t *testing.T) {
	h, _ := setupStripe(t)
	account, tok := demoUser(t, h, time.Date(time.Now().Year()-14, 1, 1, 0, 0, 0, 0, time.UTC))
	buySixTickets(t, h, tok)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	q := db.New(pool)
	execSQLArgs(t, `INSERT INTO minor_purchase_declarations(id,minor_id,ref_type,ref_id,text_version) VALUES($1,$2,'review',$3,'minor-purchase-declaration-2026-11-05')`, uuid.New(), account, uuid.New())
	if err := q.KeepMinorPurchaseDeclarationsOnWithdraw(ctx, account); err != nil {
		t.Fatal(err)
	}
	var legacyBirth, newBirth, transactions int
	queryRowArgs(t, `SELECT count(*) FILTER(WHERE text_version='minor-purchase-declaration-2026-11-05' AND birth_month IS NOT NULL),count(*) FILTER(WHERE text_version='purchase-notice-2026-11-13' AND birth_month IS NULL),count(*) FILTER(WHERE text_version='purchase-notice-2026-11-13' AND last_transaction_at IS NOT NULL) FROM minor_purchase_declarations WHERE minor_id=$1`, []any{account}, &legacyBirth, &newBirth, &transactions)
	if legacyBirth != 1 || newBirth != 1 || transactions != 1 {
		t.Fatalf("legacy=%d new=%d transactions=%d", legacyBirth, newBirth, transactions)
	}
	now := time.Now().AddDate(5, 0, 1)
	if _, err := q.DeleteExpiredMinorPurchaseDeclarations(ctx, db.DeleteExpiredMinorPurchaseDeclarationsParams{KeepUntilAge: 23, Now: now, Cutoff: now.AddDate(-5, 0, 0)}); err != nil {
		t.Fatal(err)
	}
	var remaining int
	queryRowArgs(t, `SELECT count(*) FROM minor_purchase_declarations WHERE minor_id=$1 AND text_version='minor-purchase-declaration-2026-11-05'`, []any{account}, &remaining)
	if remaining != 1 || declarationCount(t, account) != 1 {
		t.Fatal("new record must expire after five years while legacy remains")
	}
}
