package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type coachApplication struct {
	Name         string   `json:"name"`
	Tier         string   `json:"tier"`
	Events       []string `json:"events"`
	BestRecord   string   `json:"best_record"`
	Achievements string   `json:"achievements"`
	Years        int      `json:"years"`
	SnsUrls      []string `json:"sns_urls"`
	Status       string   `json:"status"`
	ReviewedAt   *string  `json:"reviewed_at"`
}

type coachMeView struct {
	Application *coachApplication `json:"application"`
}

func coachApplicationBody() map[string]any {
	return map[string]any{
		"name":         "テストコーチ",
		"tier":         "coach",
		"events":       []string{"110mh", "100m", "100m", "relay"},
		"best_record":  "100m 10秒95（2025年）",
		"achievements": "インターハイ 100m 準決勝",
		"years":        8,
		"sns_urls":     []string{"https://example-club.jp/"},
	}
}

func TestCoachApplicationFlow(t *testing.T) {
	h := setup(t)
	applicant := uuid.New()
	ta := token(t, applicant)

	rec := doJSON(t, h, http.MethodGet, "/v1/coach/me", ta, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"application":null`) {
		t.Fatalf("fresh %d %s", rec.Code, rec.Body.String())
	}

	bad := coachApplicationBody()
	bad["tier"] = "pro"
	bad["events"] = []string{"sprint"}
	bad["best_record"] = " "
	bad["years"] = 61
	delete(bad, "achievements")
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, bad)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid %d %s", rec.Code, rec.Body.String())
	}
	for _, field := range []string{"tier", "events", "best_record", "achievements", "years"} {
		if !strings.Contains(rec.Body.String(), `"`+field+`"`) {
			t.Fatalf("missing field error %s: %s", field, rec.Body.String())
		}
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, coachApplicationBody())
	if rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	var me coachMeView
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Application == nil || me.Application.Status != "pending" || strings.Join(me.Application.Events, ",") != "100m,110mh,relay" {
		t.Fatalf("applied %+v", me.Application)
	}

	// 審査待ちのうちは書き直せる。
	edit := coachApplicationBody()
	edit["years"] = 9
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, edit)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"years":9`) {
		t.Fatalf("edit %d %s", rec.Code, rec.Body.String())
	}

	// 審査待ちのうちはコーチ一覧に出ない。
	if body := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil).Body.String(); strings.Contains(body, applicant.String()) {
		t.Fatalf("pending in public list %s", body)
	}

	// 管理者でなければ一覧も審査もできない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/coach-applications", ta, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin list %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", ta, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin approve %d", rec.Code)
	}

	admin := uuid.New()
	tadmin := token(t, admin)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("admin me %d", rec.Code)
	}
	setAdmin(t, admin)

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/coach-applications", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), applicant.String()) {
		t.Fatalf("admin list %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/coach-applications?status=unknown", tadmin, nil); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad status %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+uuid.NewString()+"/approve", tadmin, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("missing %d", rec.Code)
	}

	// 却下されたら出し直せて、また審査待ちに戻る。
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/reject", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"rejected"`) {
		t.Fatalf("reject %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", tadmin, nil); rec.Code != http.StatusConflict {
		t.Fatalf("approve rejected %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, coachApplicationBody())
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if rec.Code != http.StatusOK || me.Application.Status != "pending" || me.Application.ReviewedAt != nil {
		t.Fatalf("reapply %d %s", rec.Code, rec.Body.String())
	}

	// 顔写真がないと承認できない。
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", tadmin, nil)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "顔写真") {
		t.Fatalf("approve without photo %d %s", rec.Code, rec.Body.String())
	}
	withCoachPhoto(t, applicant)
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"approved"`) {
		t.Fatalf("approve %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/coach-applications?status=approved", tadmin, nil); !strings.Contains(rec.Body.String(), applicant.String()) {
		t.Fatalf("approved list %s", rec.Body.String())
	}

	// 承認されるとコーチ一覧に出る。ログインしなくても見られる。
	rec = doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), applicant.String()) || !strings.Contains(rec.Body.String(), `"best_record":"100m 10秒95（2025年）"`) {
		t.Fatalf("public list %d %s", rec.Code, rec.Body.String())
	}

	// 承認された後は応募内容を変えられない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, edit); rec.Code != http.StatusConflict {
		t.Fatalf("edit approved %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/coach/me", ta, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Application == nil || me.Application.Status != "approved" || me.Application.Years != 8 {
		t.Fatalf("me after approve %+v", me.Application)
	}
}

func TestCoachPhoto(t *testing.T) {
	var deleted []string
	asset := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = append(deleted, r.URL.Query()["public_ids[]"]...)
			w.WriteHeader(http.StatusOK)
			return
		}
		if !strings.Contains(r.URL.Path, "/resources/image/upload/track-talk/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resource_type":"image","format":"jpg","bytes":1024}`))
	}))
	defer asset.Close()
	t.Setenv("CLOUDINARY_CLOUD_NAME", "demo")
	t.Setenv("CLOUDINARY_API_KEY", "key")
	t.Setenv("CLOUDINARY_API_SECRET", "secret")
	t.Setenv("CLOUDINARY_API_BASE", asset.URL)

	h := setup(t)
	applicant := uuid.New()
	ta := token(t, applicant)
	upload := func() string {
		t.Helper()
		rec := doJSON(t, h, http.MethodPost, "/v1/uploads/image", ta, map[string]any{})
		var signed struct {
			PublicID string `json:"public_id"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &signed)
		if rec.Code != http.StatusOK || signed.PublicID == "" {
			t.Fatalf("sign %d %s", rec.Code, rec.Body.String())
		}
		return signed.PublicID
	}

	// 応募する前は写真を登録できない。
	first := upload()
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/photo", ta, map[string]any{"photo_public_id": first}); rec.Code != http.StatusConflict {
		t.Fatalf("before apply %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/photo", ta, map[string]any{"photo_public_id": "../x"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad id %d", rec.Code)
	}
	rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/photo", ta, map[string]any{"photo_public_id": first})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "c_fill,g_face") {
		t.Fatalf("photo %d %s", rec.Code, rec.Body.String())
	}

	// 差し替えると、前の写真は Cloudinary から消す。
	second := upload()
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/photo", ta, map[string]any{"photo_public_id": second}); rec.Code != http.StatusOK {
		t.Fatalf("replace %d", rec.Code)
	}
	if len(deleted) != 1 || deleted[0] != first {
		t.Fatalf("deleted %v", deleted)
	}

	// 他人の投稿の写真の ID を登録してから差し替えても、その写真は消さない。
	others := upload()
	tb := token(t, uuid.New())
	grantUpload(t, h, tb, others)
	if rec := doJSON(t, h, http.MethodPost, "/v1/posts", tb, map[string]any{"title": "写真の投稿", "body": "", "image_public_id": others}); rec.Code != http.StatusCreated {
		t.Fatalf("other's post %d %s", rec.Code, rec.Body.String())
	}
	grantUpload(t, h, ta, others)
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/photo", ta, map[string]any{"photo_public_id": others}); rec.Code != http.StatusOK {
		t.Fatalf("set other's photo %d", rec.Code)
	}
	deleted = nil
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/photo", ta, map[string]any{"photo_public_id": second}); rec.Code != http.StatusOK {
		t.Fatalf("replace again %d", rec.Code)
	}
	if len(deleted) != 0 {
		t.Fatalf("other's photo deleted %v", deleted)
	}

	// 承認されると、コーチ一覧に写真が出る。
	admin := uuid.New()
	tadmin := token(t, admin)
	doJSON(t, h, http.MethodGet, "/v1/me", tadmin, nil)
	setAdmin(t, admin)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d", rec.Code)
	}
	if body := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil).Body.String(); !strings.Contains(body, second) {
		t.Fatalf("public list %s", body)
	}

	// 運営は写真を外せる。コーチ本人は外せず、外すと Cloudinary からも消える。
	if rec := doJSON(t, h, http.MethodDelete, "/v1/admin/coaching/coaches/"+applicant.String()+"/photo", ta, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("remove by coach %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodDelete, "/v1/admin/coaching/coaches/"+uuid.NewString()+"/photo", tadmin, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("remove unknown %d", rec.Code)
	}
	deleted = nil
	if rec := doJSON(t, h, http.MethodDelete, "/v1/admin/coaching/coaches/"+applicant.String()+"/photo", tadmin, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("remove %d", rec.Code)
	}
	if len(deleted) != 1 || deleted[0] != second {
		t.Fatalf("deleted after remove %v", deleted)
	}
	if body := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil).Body.String(); strings.Contains(body, second) || !strings.Contains(body, `"photo_url":""`) {
		t.Fatalf("photo still listed %s", body)
	}
}

func TestCoachApplicationSnsUrls(t *testing.T) {
	h := setup(t)
	applicant := uuid.New()
	ta := token(t, applicant)

	// URL でないもの、http/https 以外、4件は受け付けない。
	for _, urls := range [][]string{
		{"instagram.com/testcoach"},
		{"ftp://example.com/testcoach"},
		{"https://example.com/1", "https://example.com/2", "https://example.com/3", "https://example.com/4"},
	} {
		body := coachApplicationBody()
		body["sns_urls"] = urls
		rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, body)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"sns_urls"`) {
			t.Fatalf("invalid sns %v: %d %s", urls, rec.Code, rec.Body.String())
		}
	}

	// 空行と重複は除いて保存し、自分の応募に返る。
	body := coachApplicationBody()
	body["sns_urls"] = []string{" https://www.instagram.com/testcoach/ ", "", "https://www.instagram.com/testcoach/", "https://x.com/testcoach"}
	rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply with sns %d %s", rec.Code, rec.Body.String())
	}
	var me coachMeView
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Application == nil || strings.Join(me.Application.SnsUrls, ",") != "https://www.instagram.com/testcoach/,https://x.com/testcoach" {
		t.Fatalf("sns saved %+v", me.Application)
	}

	// SNSまたはホームページは必須。空白だけでも受け付けない。
	for _, urls := range [][]string{nil, {}, {" ", ""}} {
		body := coachApplicationBody()
		body["sns_urls"] = urls
		rec = doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, body)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"sns_urls"`) {
			t.Fatalf("apply without sns %d %s", rec.Code, rec.Body.String())
		}
	}
	// クラブチームのホームページだけでも応募できる。
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, coachApplicationBody())
	if rec.Code != http.StatusOK {
		t.Fatalf("apply with website %d %s", rec.Code, rec.Body.String())
	}
}

func TestCoachApplicationNameWithoutYears(t *testing.T) {
	h := setup(t)
	applicant := uuid.New()
	ta := token(t, applicant)
	body := coachApplicationBody()
	delete(body, "years")
	body["name"] = "  山田 太郎  "
	rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, body)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"山田 太郎"`) {
		t.Fatalf("name without years %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/coach/me", ta, nil)
	if !strings.Contains(rec.Body.String(), `"name":"山田 太郎"`) {
		t.Fatalf("saved name %s", rec.Body.String())
	}
	withCoachPhoto(t, applicant)
	admin := uuid.New()
	tadmin := token(t, admin)
	doJSON(t, h, http.MethodGet, "/v1/me", tadmin, nil)
	setAdmin(t, admin)
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", tadmin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil)
	if !strings.Contains(rec.Body.String(), `"nickname":"山田 太郎"`) {
		t.Fatalf("public name %s", rec.Body.String())
	}
}

func TestCoachApplicationNameValidation(t *testing.T) {
	h := setup(t)
	ta := token(t, uuid.New())
	for _, name := range []string{" ", strings.Repeat("あ", 81), "https://example.com"} {
		body := coachApplicationBody()
		body["name"] = name
		rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, body)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"name"`) {
			t.Fatalf("invalid name %d %s", rec.Code, rec.Body.String())
		}
	}
}

func TestLegacyCoachApplicationPreservesName(t *testing.T) {
	h := setup(t)
	ta := token(t, uuid.New())
	body := coachApplicationBody()
	delete(body, "name")
	rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("legacy application %d %s", rec.Code, rec.Body.String())
	}
	body["name"] = "コーチの公開名"
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("new application %d %s", rec.Code, rec.Body.String())
	}
	delete(body, "name")
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, body)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"コーチの公開名"`) {
		t.Fatalf("legacy update preserves name %d %s", rec.Code, rec.Body.String())
	}
}
