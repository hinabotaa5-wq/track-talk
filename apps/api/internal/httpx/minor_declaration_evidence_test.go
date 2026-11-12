package httpx_test

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// 購入ごとの宣言の記録には、証跡として、その時の宣言文と注意書きの文面・取引の金額とコーチ・
// その時の生まれた年月・画面での表示とチェックの時刻が残る。端末の時刻が狂っていれば時刻だけ残さない。
func TestMinorDeclarationRecordsEvidence(t *testing.T) {
	allowPayPayForTest(t)
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, admin)
	birth := time.Date(time.Now().Year()-14, 3, 1, 0, 0, 0, 0, time.UTC)
	minor, tm := demoUser(t, h, birth)

	// 依頼を作る時の宣言は、時刻なしで送る（古い画面から来た場合と同じ）。
	id := chosenReview(t, h, tm, coach.String(), "paypay", false)

	// 支払い画面を開き直す時に、表示とチェックの時刻を一緒に送る。
	shown := time.Now().Add(-2 * time.Minute).UTC().Truncate(time.Second)
	checked := shown.Add(30 * time.Second)
	rec := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/checkout", tm, map[string]any{
		"method": "paypay", "guardian_permission": true,
		"guardian_permission_shown_at": shown.Format(time.RFC3339), "guardian_permission_checked_at": checked.Format(time.RFC3339),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("reopen %d %s", rec.Code, rec.Body.String())
	}
	// 端末の時計が大きく狂っている時刻は残さない（宣言そのものは受け付ける）。
	rec = doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/checkout", tm, map[string]any{
		"method": "paypay", "guardian_permission": true,
		"guardian_permission_shown_at": "2001-01-01T00:00:00Z", "guardian_permission_checked_at": time.Now().Add(time.Hour).Format(time.RFC3339),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("reopen with bad clock %d %s", rec.Code, rec.Body.String())
	}

	var total, evidence, withTimes, partial int
	queryRowArgs(t, `SELECT count(*),
			count(*) FILTER (WHERE declaration_text = '保護者に許可をもらいました'
				AND notice_text = '未成年の方は、購入する前に保護者の許可を得てください。'
				AND amount_yen = 500 AND coach_id = $3 AND to_char(declared_birth_month, 'YYYY-MM') = $4),
			count(*) FILTER (WHERE client_shown_at = $5 AND client_checked_at = $6),
			count(*) FILTER (WHERE (client_shown_at IS NULL) <> (client_checked_at IS NULL))
		FROM minor_purchase_declarations WHERE minor_id = $1 AND ref_type = 'review' AND ref_id = $2`,
		[]any{minor, id, coach, birth.Format("2006-01"), shown, checked}, &total, &evidence, &withTimes, &partial)
	if total != 3 || evidence != 3 || withTimes != 1 || partial != 0 {
		t.Fatalf("declarations total=%d evidence=%d with times=%d partial=%d", total, evidence, withTimes, partial)
	}
}

// 記録に写す宣言の文面は、画面（apps/web/lib/minor-purchase.ts）に出す文面と同じでなければならない。
func TestMinorDeclarationTextMatchesWeb(t *testing.T) {
	src, err := os.ReadFile("../../../web/lib/minor-purchase.ts")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"保護者に許可をもらいました"`, `"未成年の方は、購入する前に保護者の許可を得てください。"`} {
		if !strings.Contains(string(src), want) {
			t.Fatalf("web text %s not found", want)
		}
	}
}
