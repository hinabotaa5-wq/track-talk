package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// applyApproveCoach は、応募→顔写真→承認まで進めたコーチを作り、管理者のトークンを返す。
func applyApproveCoach(t *testing.T, h http.Handler, applicant uuid.UUID, ta string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, coachApplicationBody())
	if rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	withCoachPhoto(t, applicant)
	admin := uuid.New()
	tadmin := token(t, admin)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("admin me %d %s", rec.Code, rec.Body.String())
	}
	setAdmin(t, admin)
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", tadmin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve %d %s", rec.Code, rec.Body.String())
	}
	return tadmin
}

func profileUpdateBody() map[string]any {
	return map[string]any{
		"name":         "テストコーチ",
		"events":       []string{"100m"},
		"best_record":  "100m 10秒95（2025年）",
		"achievements": "インターハイ 100m 準決勝",
		"sns_urls":     []string{"https://example-club.jp/"},
		"hitokoto":     "よろしくお願いします",
	}
}

// 承認済みコーチの自己編集：即時反映・status 不変・変更ありだけ通知・変更なしは通知なし。
func TestCoachProfileUpdate(t *testing.T) {
	h := setup(t)
	applicant := uuid.New()
	ta := token(t, applicant)
	tadmin := applyApproveCoach(t, h, applicant, ta)

	var before struct {
		Application *struct {
			Status    string `json:"status"`
			AppliedAt string `json:"applied_at"`
		} `json:"application"`
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/coach/me", ta, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &before)

	// 承認済み以外（審査待ち）は 403。別コーチで確かめる。
	pending := uuid.New()
	tpending := token(t, pending)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tpending, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("pending apply %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/profile", tpending, profileUpdateBody()); rec.Code != http.StatusForbidden {
		t.Fatalf("pending update %d %s", rec.Code, rec.Body.String())
	}

	mails := operatorMailCount()
	rec = doJSON(t, h, http.MethodPut, "/v1/coach/me/profile", ta, profileUpdateBody())
	if rec.Code != http.StatusOK {
		t.Fatalf("update %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"status":"approved"`, `"hitokoto":"よろしくお願いします"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("update response missing %s: %s", want, body)
		}
	}
	// status・applied_at は変わらない（再審査に回さない）。
	var after struct {
		Application *struct {
			Status    string `json:"status"`
			AppliedAt string `json:"applied_at"`
		} `json:"application"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &after)
	if after.Application.Status != "approved" || after.Application.AppliedAt != before.Application.AppliedAt {
		t.Fatalf("status/applied_at changed: %+v", after.Application)
	}
	// 変更ありでは通知が1通だけ送られる。
	if got := operatorMailCount() - mails; got != 1 {
		t.Fatalf("mails after update = %d, want 1", got)
	}
	m := lastOperatorMail(t)
	if m.Subject != "コーチがプロフィールを変更しました" {
		t.Fatalf("subject %q", m.Subject)
	}
	for _, want := range []string{"テストコーチ", "ひとこと", "JST", "/admin"} {
		if !strings.Contains(m.Text, want) {
			t.Fatalf("mail missing %q: %q", want, m.Text)
		}
	}
	for _, leak := range []string{"インターハイ 100m 準決勝", "100m 10秒95（2025年）", "よろしくお願いします"} {
		if strings.Contains(m.Text, leak) {
			t.Fatalf("mail contains profile prose %q: %q", leak, m.Text)
		}
	}

	// 公開一覧と本人の取得に即時反映される。
	rec = doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hitokoto":"よろしくお願いします"`) {
		t.Fatalf("public list %d %s", rec.Code, rec.Body.String())
	}
	// 管理一覧にもひとことが載る。
	rec = doJSON(t, h, http.MethodGet, "/v1/admin/coach-applications?status=approved", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hitokoto":"よろしくお願いします"`) {
		t.Fatalf("admin list %d %s", rec.Code, rec.Body.String())
	}

	// 変更なしの保存では通知を送らない。
	rec = doJSON(t, h, http.MethodPut, "/v1/coach/me/profile", ta, profileUpdateBody())
	if rec.Code != http.StatusOK {
		t.Fatalf("no-change update %d %s", rec.Code, rec.Body.String())
	}
	if got := operatorMailCount() - mails; got != 1 {
		t.Fatalf("mails after no-change update = %d, want 1", got)
	}

	// ひとことだけ変えると、変更項目はひとことだけになる。
	edit := profileUpdateBody()
	edit["hitokoto"] = "がんばりましょう"
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/profile", ta, edit); rec.Code != http.StatusOK {
		t.Fatalf("hitokoto edit %d %s", rec.Code, rec.Body.String())
	}
	if got := operatorMailCount() - mails; got != 2 {
		t.Fatalf("mails after hitokoto edit = %d, want 2", got)
	}
	m = lastOperatorMail(t)
	if !strings.Contains(m.Text, "ひとこと") {
		t.Fatalf("mail missing hitokoto field: %q", m.Text)
	}
	for _, not := range []string{"名前", "担当種目", "記録", "成績や指導経験", "SNS"} {
		if strings.Contains(m.Text, not) {
			t.Fatalf("mail lists unchanged field %q: %q", not, m.Text)
		}
	}

	// 停止中のコーチは変えられない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coaches/"+applicant.String()+"/suspend", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("suspend %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/profile", ta, profileUpdateBody()); rec.Code != http.StatusForbidden {
		t.Fatalf("suspended update %d %s", rec.Code, rec.Body.String())
	}
}

// 自己編集の検証エラーは項目別に返る。tier・years は対象外で送ると 400。
func TestCoachProfileUpdateValidation(t *testing.T) {
	h := setup(t)
	applicant := uuid.New()
	ta := token(t, applicant)
	applyApproveCoach(t, h, applicant, ta)

	cases := []struct {
		name   string
		mutate func(map[string]any)
		field  string
	}{
		{"種目なし", func(b map[string]any) { b["events"] = []string{} }, "events"},
		{"SNSなし", func(b map[string]any) { b["sns_urls"] = []string{} }, "sns_urls"},
		{"SNS4件", func(b map[string]any) {
			b["sns_urls"] = []string{"a", "b", "c", "d"}
		}, "sns_urls"},
		{"ひとこと101文字", func(b map[string]any) { b["hitokoto"] = strings.Repeat("あ", 101) }, "hitokoto"},
		{"ひとこと改行", func(b map[string]any) { b["hitokoto"] = "1行目\n2行目" }, "hitokoto"},
		{"ひとこと連絡先", func(b map[string]any) { b["hitokoto"] = "090-1234-5678" }, "hitokoto"},
		{"名前空欄", func(b map[string]any) { b["name"] = " " }, "name"},
	}
	for _, tc := range cases {
		body := profileUpdateBody()
		tc.mutate(body)
		rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/profile", ta, body)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"`+tc.field+`"`) {
			t.Fatalf("%s: %d %s", tc.name, rec.Code, rec.Body.String())
		}
	}
	// ひとことは空欄でも保存できる。
	body := profileUpdateBody()
	body["hitokoto"] = ""
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/profile", ta, body); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"hitokoto":""`) {
		t.Fatalf("empty hitokoto %d %s", rec.Code, rec.Body.String())
	}
	// tier・years は対象外。送ると未知の項目として 400。
	body = profileUpdateBody()
	body["tier"] = "coach"
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/profile", ta, body); rec.Code != http.StatusBadRequest {
		t.Fatalf("tier %d %s", rec.Code, rec.Body.String())
	}
	body = profileUpdateBody()
	body["years"] = 8
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/profile", ta, body); rec.Code != http.StatusBadRequest {
		t.Fatalf("years %d %s", rec.Code, rec.Body.String())
	}
}

// 応募時のひとこと：保存・審査待ちの更新・空欄・検証。
func TestCoachApplicationHitokoto(t *testing.T) {
	h := setup(t)
	applicant := uuid.New()
	ta := token(t, applicant)

	with := coachApplicationBody()
	with["hitokoto"] = "よろしくお願いします"
	rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, with)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hitokoto":"よろしくお願いします"`) {
		t.Fatalf("apply with hitokoto %d %s", rec.Code, rec.Body.String())
	}

	// 審査待ちの再送信で更新できる。
	with["hitokoto"] = "がんばりましょう"
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, with)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hitokoto":"がんばりましょう"`) {
		t.Fatalf("resubmit hitokoto %d %s", rec.Code, rec.Body.String())
	}

	// 空欄でも応募できる。
	without := coachApplicationBody()
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, without)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hitokoto":""`) {
		t.Fatalf("apply without hitokoto %d %s", rec.Code, rec.Body.String())
	}

	// 101文字は項目別エラーになり、保存されない。
	bad := coachApplicationBody()
	bad["hitokoto"] = strings.Repeat("あ", 101)
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, bad)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"hitokoto"`) {
		t.Fatalf("long hitokoto %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/coach/me", ta, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hitokoto":""`) {
		t.Fatalf("hitokoto changed on invalid submit %d %s", rec.Code, rec.Body.String())
	}
}
