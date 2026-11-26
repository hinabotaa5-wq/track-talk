package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 購入前チェックは未成年（の可能性がある人）だけに求める（本体 #1045）。
//   - birth_month がある人は月精度の isMinor（T7）。birth_year だけの人は「日本時間の今年 − 生まれた年 ≤ 18」（T6）。
//   - 年の情報がない人（API を直接呼んだ場合）は安全側でチェックを求める（T10）。
//   - チェックを求める人は未チェックで止め、チェックすると新版で記録する（T9）。成人と確定できる人は止めず、記録も作らない（T8）。
func TestPurchaseNoticeOnlyForMinors(t *testing.T) {
	jstNow := time.Now().In(time.FixedZone("JST", 9*60*60))
	year := jstNow.Year()
	// 月精度では成人（18歳になった月の翌月1日を過ぎている）だが、年の差は 18 になりうる人。月精度が優先される。
	adultByMonth := time.Date(jstNow.Year()-18, jstNow.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -2, 0)
	cases := []struct {
		name       string
		birthMonth *time.Time
		birthYear  *int
		required   bool
	}{
		{name: "unregistered", required: true},
		{name: "adult_month", birthMonth: ptrTime(time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC)), required: false},
		{name: "minor_month", birthMonth: ptrTime(time.Date(year-14, 1, 1, 0, 0, 0, 0, time.UTC)), required: true},
		{name: "year_diff_17", birthYear: ptrInt(year - 17), required: true},
		{name: "year_diff_18", birthYear: ptrInt(year - 18), required: true},
		{name: "year_diff_19", birthYear: ptrInt(year - 19), required: false},
		{name: "month_wins_over_year", birthMonth: &adultByMonth, birthYear: ptrInt(adultByMonth.Year()), required: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeCoachingCloudinary(t)
			h, f := setupCoachChoice(t)
			admin := adminToken(t, h)
			coach, _ := onDutyCoach(t, h, admin)
			account, tok := demoUser(t, h, time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC))
			execSQLArgs(t, `UPDATE profiles SET birth_month=$2, birth_year=$3 WHERE id=$1`, account, c.birthMonth, c.birthYear)

			// 画面に出すかどうかの判定は、/v1/me と /v1/me/coaching が API の判定と同じ値を返す。
			for _, path := range []string{"/v1/me", "/v1/me/coaching"} {
				me := doJSON(t, h, http.MethodGet, path, tok, nil)
				var out struct {
					Required *bool `json:"purchase_notice_required"`
				}
				_ = json.Unmarshal(me.Body.Bytes(), &out)
				if me.Code != http.StatusOK || out.Required == nil || *out.Required != c.required {
					t.Fatalf("%s purchase_notice_required: %d %s", path, me.Code, me.Body.String())
				}
			}

			path := "/v1/coaching/requests"
			body := reviewBodyFor(t, h, tok)
			body["method"] = "card"
			body["requested_coach_id"] = coach.String()
			delete(body, "guardian_permission")
			if c.required {
				sessions := sessionCount(f)
				for _, checked := range []bool{false, true} {
					if checked {
						body["guardian_permission"] = false
					}
					rec := doJSON(t, h, http.MethodPost, path, tok, body)
					if rec.Code != 422 || sessionCount(f) != sessions || !strings.Contains(rec.Body.String(), "guardian_permission_required") {
						t.Fatalf("unchecked: %d %s", rec.Code, rec.Body.String())
					}
				}
				body["guardian_permission"] = true
				body["guardian_permission_checked_at"] = time.Now().UTC().Format(time.RFC3339)
			}
			rec := doJSON(t, h, http.MethodPost, path, tok, body)
			if rec.Code != 200 && rec.Code != 201 {
				t.Fatalf("purchase: %d %s", rec.Code, rec.Body.String())
			}
			var count, all int
			queryRowArgs(t, `SELECT count(*) FROM minor_purchase_declarations WHERE minor_id=$1 AND ref_type='review' AND text_version='purchase-notice-2026-11-26' AND ref_id IS NOT NULL AND declared_at IS NOT NULL AND client_checked_at IS NOT NULL AND declared_birth_month IS NULL`, []any{account}, &count)
			queryRowArgs(t, `SELECT count(*) FROM minor_purchase_declarations WHERE minor_id=$1`, []any{account}, &all)
			want := 0
			if c.required {
				want = 1
			}
			if count != want || all != want {
				t.Fatalf("evidence count=%d all=%d want=%d", count, all, want)
			}
		})
	}
}

func ptrTime(v time.Time) *time.Time { return &v }
func ptrInt(v int) *int              { return &v }

// 既存の23歳ルールを維持し、新規の記録には生年月を写さず最後のチケット取引から5年を適用する。
func TestPurchaseNoticeRetentionPreservesLegacyRecords(t *testing.T) {
	h, _ := setupStripe(t)
	account, tok := demoUser(t, h, time.Date(time.Now().Year()-14, 1, 1, 0, 0, 0, 0, time.UTC))
	purchase := buySixTickets(t, h, tok)
	// 販売終了（本体 #1031）の前の購入で残っていた、購入のたびの注意事項の記録（新しい形式）。
	execSQLArgs(t, `INSERT INTO minor_purchase_declarations(id,minor_id,ref_type,ref_id,text_version) VALUES($1,$2,'ticket_purchase',$3,'purchase-notice-2026-11-13')`, uuid.New(), account, purchase)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	q := db.New(pool)
	execSQLArgs(t, `INSERT INTO minor_purchase_declarations(id,minor_id,ref_type,ref_id,text_version) VALUES($1,$2,'review',$3,'minor-purchase-declaration-2026-11-05')`, uuid.New(), account, uuid.New())
	// 未成年だけに出す新版（本体 #1045）も、保持の数え方は 2026-11-13 版と同じ（生年月を写さず、最後の完了した取引から5年）。
	execSQLArgs(t, `INSERT INTO minor_purchase_declarations(id,minor_id,ref_type,ref_id,text_version) VALUES($1,$2,'review',$3,'purchase-notice-2026-11-26')`, uuid.New(), account, uuid.New())
	if err := q.KeepMinorPurchaseDeclarationsOnWithdraw(ctx, account); err != nil {
		t.Fatal(err)
	}
	var v2Birth, v2Transactions int
	queryRowArgs(t, `SELECT count(*) FILTER(WHERE birth_month IS NULL),count(*) FILTER(WHERE last_transaction_at IS NOT NULL) FROM minor_purchase_declarations WHERE minor_id=$1 AND text_version='purchase-notice-2026-11-26'`, []any{account}, &v2Birth, &v2Transactions)
	if v2Birth != 1 || v2Transactions != 1 {
		t.Fatalf("v2 birth=%d transactions=%d", v2Birth, v2Transactions)
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
