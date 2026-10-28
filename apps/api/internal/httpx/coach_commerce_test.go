package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/hinata/track-talk/apps/api/internal/config"
)

const (
	testLegalName = "山田太郎"
	testAddress   = "東京都千代田区千代田1-1-1"
	testPhone     = "090-1234-5678"
)

func registerCommerceIdentity(t *testing.T, h http.Handler, tc string) {
	t.Helper()
	rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/commerce/identity", tc, map[string]string{
		"legal_name": testLegalName, "address": testAddress, "phone": testPhone,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("register identity %d %s", rec.Code, rec.Body.String())
	}
}

func publicPlan(t *testing.T, h http.Handler, plan string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/plans/"+plan, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("public plan %d %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// 住所などは暗号化して保存し、表記要のコーチのプランにだけ出す。表記不要のコーチの分は公開の API に出さない。
func TestCoachCommerceDisclosureShownOnlyWhenRequired(t *testing.T) {
	h := setup(t)
	coach, tc, plan := planCoach(t, h, 5)

	// DB には平文で残らない。
	var raw string
	queryRowArgs(t, `SELECT encode(legal_name_enc,'escape') || encode(address_enc,'escape') || encode(phone_enc,'escape') FROM coach_legal_identities WHERE coach_id=$1`, []any{coach}, &raw)
	if strings.Contains(raw, "山田") || strings.Contains(raw, "千代田") || strings.Contains(raw, "1234") {
		t.Fatal("identity stored in plain text")
	}

	// 既定は表記要。プランのページにフル表記が出る。
	body := publicPlan(t, h, plan)
	for _, want := range []string{`"required":true`, testLegalName, testAddress, testPhone} {
		if !strings.Contains(body, want) {
			t.Fatalf("required plan missing %s: %s", want, body)
		}
	}

	// チェックリストが1つでも欠けたら不要にできない。
	rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/commerce/exemption", tc, map[string]any{"exempt": true, "checklist_version": 1, "checked": []bool{true, false, true}})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("partial checklist %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPut, "/v1/coach/me/commerce/exemption", tc, map[string]any{"exempt": true, "checklist_version": 1, "checked": []bool{true, true, true}})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"required":false`) {
		t.Fatalf("exempt %d %s", rec.Code, rec.Body.String())
	}
	// 本人には自分の登録情報が見える。
	if !strings.Contains(rec.Body.String(), testAddress) {
		t.Fatalf("own identity hidden: %s", rec.Body.String())
	}

	// 表記不要のコーチの登録情報は、公開の API のどこにも出ない。
	body = publicPlan(t, h, plan)
	if !strings.Contains(body, `"commerce":{"required":false}`) {
		t.Fatalf("exempt plan commerce: %s", body)
	}
	coaches := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil).Body.String()
	for _, leak := range []string{testLegalName, testAddress, testPhone} {
		if strings.Contains(body, leak) || strings.Contains(coaches, leak) {
			t.Fatalf("identity leaked in public API: %s", leak)
		}
	}

	// 運営が表記要に戻すと、本人は不要に申告し直せない。
	admin := adminToken(t, h)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coaches/"+coach.String()+"/commerce-required", admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("force %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPut, "/v1/coach/me/commerce/exemption", tc, map[string]any{"exempt": true, "checklist_version": 1, "checked": []bool{true, true, true}})
	if rec.Code != http.StatusConflict {
		t.Fatalf("exempt after force %d %s", rec.Code, rec.Body.String())
	}
	if body := publicPlan(t, h, plan); !strings.Contains(body, testAddress) {
		t.Fatalf("forced plan should disclose: %s", body)
	}
}

// プランを公開する前に、全コーチが氏名・住所・電話番号を登録する。
func TestCoachCommerceIdentityRequiredBeforePublishing(t *testing.T) {
	h := setup(t)
	_, tc, _ := planCoach(t, h, 5)
	execSQL(t, `DELETE FROM coach_legal_identities`)
	body := planBody()
	rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, body)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "特定商取引法の表記") {
		t.Fatalf("publish without identity %d %s", rec.Code, rec.Body.String())
	}
	body["published"] = false
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, body); rec.Code != http.StatusCreated {
		t.Fatalf("draft without identity %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/commerce/identity", tc, map[string]string{"legal_name": "", "address": "", "phone": "abc"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid identity %d %s", rec.Code, rec.Body.String())
	}
}

// 運営は目的を書いたときだけ登録情報を見られ、見た記録が残る。
func TestAdminViewsCoachIdentityWithAccessLog(t *testing.T) {
	h := setup(t)
	coach, tc, _ := planCoach(t, h, 5)
	path := "/v1/admin/coaching/coaches/" + coach.String() + "/commerce-identity"
	if rec := doJSON(t, h, http.MethodPost, path, tc, map[string]string{"purpose": "開示請求"}); rec.Code != http.StatusForbidden {
		t.Fatalf("coach viewing identity %d", rec.Code)
	}
	admin := adminToken(t, h)
	if rec := doJSON(t, h, http.MethodPost, path, admin, map[string]string{"purpose": ""}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("no purpose %d", rec.Code)
	}
	rec := doJSON(t, h, http.MethodPost, path, admin, map[string]string{"purpose": "利用者からの開示請求(問い合わせ番号123)"})
	var out struct {
		Identity struct {
			Address string `json:"address"`
		} `json:"identity"`
		AccessLogs []struct {
			Purpose string `json:"purpose"`
		} `json:"access_logs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || rec.Code != 200 || out.Identity.Address != testAddress || len(out.AccessLogs) != 1 {
		t.Fatalf("admin view %d %s", rec.Code, rec.Body.String())
	}
}

// 鍵がない環境では、登録情報を保存しない(平文で入れない)。
func TestCoachCommerceRefusesWithoutKey(t *testing.T) {
	h := setupConfig(t, func(c *config.Config) { c.CoachIdentityKey = "" })
	_, tc := onDutyCoach(t, h, adminToken(t, h))
	rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/commerce/identity", tc, map[string]string{"legal_name": testLegalName, "address": testAddress, "phone": testPhone})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("without key %d %s", rec.Code, rec.Body.String())
	}
}
