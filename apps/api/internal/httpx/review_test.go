package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/jackc/pgx/v5/pgxpool"
)

type reviewRequestView struct {
	ID       string   `json:"id"`
	Tier     string   `json:"tier"`
	Event    string   `json:"event"`
	Focus    []string `json:"focus"`
	Status   string   `json:"status"`
	DueAt    string   `json:"due_at"`
	VideoURL string   `json:"video_url"`
}

// fakeCoachingCloudinary は、非公開でアップロードされた添削の動画があるように答える。
func fakeCoachingCloudinary(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/resources/video/authenticated/track-talk-coaching/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resource_type":"video","type":"authenticated","format":"mp4","bytes":1024,"duration":12}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("CLOUDINARY_CLOUD_NAME", "demo")
	t.Setenv("CLOUDINARY_API_KEY", "key")
	t.Setenv("CLOUDINARY_API_SECRET", "secret")
	t.Setenv("CLOUDINARY_API_BASE", srv.URL)
}

func execSQLArgs(t *testing.T, sql string, args ...any) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		t.Fatal(err)
	}
}

func reviewBody() map[string]any {
	return map[string]any{
		"tier":            "coach",
		"event":           "100m",
		"focus":           []string{"rhythm", "start", "start"},
		"video_public_id": "track-talk-coaching/" + uuid.NewString(),
	}
}

func newDemoCodeFor(t *testing.T, h http.Handler, adminToken string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/demo-codes", adminToken, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("demo code %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Code) != 8 {
		t.Fatalf("code %q", out.Code)
	}
	return out.Code
}

func adminToken(t *testing.T, h http.Handler) string {
	t.Helper()
	admin := uuid.New()
	ta := token(t, admin)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", ta, nil); rec.Code != http.StatusOK {
		t.Fatalf("admin me %d", rec.Code)
	}
	setAdmin(t, admin)
	return ta
}

func TestReviewRequestNeedsDemoCode(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	user := uuid.New()
	tu := token(t, user)
	tadmin := adminToken(t, h)

	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/demo-codes", tu, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin code %d", rec.Code)
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBody())
	if rec.Code != http.StatusForbidden {
		t.Fatalf("before demo %d %s", rec.Code, rec.Body.String())
	}

	code := newDemoCodeFor(t, h, tadmin)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/demo", tu, map[string]any{"code": "ZZZZZZZZ"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("wrong code %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/demo", tu, map[string]any{"code": strings.ToLower(code[:4]) + "-" + code[4:]}); rec.Code != http.StatusOK {
		t.Fatalf("join %d %s", rec.Code, rec.Body.String())
	}
	// 1つのコードは1人だけが使える。
	other := token(t, uuid.New())
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/demo", other, map[string]any{"code": code}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("reuse %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/admin/coaching/demo-codes", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), code) || !strings.Contains(rec.Body.String(), `"used_at":"`) {
		t.Fatalf("codes %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/coaching", tu, nil)
	if !strings.Contains(rec.Body.String(), `"demo":true`) {
		t.Fatalf("me demo %s", rec.Body.String())
	}

	// 生まれた年月がないと送れない。
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBody())
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "birth_month_required") {
		t.Fatalf("no birth month %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/birth-month", tu, map[string]any{"birth_month": "1995-04"}); rec.Code != http.StatusOK {
		t.Fatalf("birth month %d", rec.Code)
	}

	bad := reviewBody()
	bad["tier"] = "pro"
	bad["event"] = "marathon"
	bad["focus"] = []string{}
	bad["video_public_id"] = "track-talk/" + uuid.NewString()
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, bad)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid %d", rec.Code)
	}
	for _, field := range []string{"tier", "event", "focus", "video_public_id"} {
		if !strings.Contains(rec.Body.String(), `"`+field+`"`) {
			t.Fatalf("missing %s: %s", field, rec.Body.String())
		}
	}

	body := reviewBody()
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created reviewRequestView
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Status != "queued" || strings.Join(created.Focus, ",") != "start,rhythm" || !dueAtNoonJST(created.DueAt) || !strings.Contains(created.VideoURL, "/video/authenticated/s--") {
		t.Fatalf("created %+v", created)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body); rec.Code != http.StatusConflict {
		t.Fatalf("same video %d", rec.Code)
	}

	// 他の人には見えない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+created.ID, other, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("other get %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/coaching/requests", tu, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), created.ID) {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
}

func TestMinorReviewRequestWaitsForGuardian(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	minor := uuid.New()
	tm := token(t, minor)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/demo", tm, map[string]any{"code": newDemoCodeFor(t, h, tadmin)}); rec.Code != http.StatusOK {
		t.Fatalf("join %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/birth-month", tm, map[string]any{"birth_month": minorBirthMonth()}); rec.Code != http.StatusOK {
		t.Fatalf("birth month %d", rec.Code)
	}
	// 1本目は同意を待たずに送れる。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tm, reviewBody()); rec.Code != http.StatusCreated {
		t.Fatalf("first %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tm, reviewBody())
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "guardian_pending") {
		t.Fatalf("second before consent %d %s", rec.Code, rec.Body.String())
	}
	execSQLArgs(t, `INSERT INTO guardian_consents (minor_id, token_hash, status, guardian_name, relationship, consented_at)
		VALUES ($1, $2, 'consented', '保護者', 'mother', now())`, minor, uuid.NewString())
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tm, reviewBody()); rec.Code != http.StatusCreated {
		t.Fatalf("after consent %d %s", rec.Code, rec.Body.String())
	}
	execSQLArgs(t, `UPDATE guardian_consents SET status = 'declined', declined_at = now() WHERE minor_id = $1`, minor)
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tm, reviewBody())
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "guardian_declined") {
		t.Fatalf("declined %d %s", rec.Code, rec.Body.String())
	}
}

func TestCoachDuty(t *testing.T) {
	h := setup(t)
	coach := uuid.New()
	tc := token(t, coach)
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": true}); rec.Code != http.StatusForbidden {
		t.Fatalf("not coach %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tc, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": true}); rec.Code != http.StatusForbidden {
		t.Fatalf("pending %d", rec.Code)
	}
	tadmin := adminToken(t, h)
	withCoachPhoto(t, coach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+coach.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d", rec.Code)
	}
	// 通知を受け取れる端末がないと受付中にできない。
	rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": true})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("no device %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"daily_cap": 21}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("cap %d", rec.Code)
	}
	execSQLArgs(t, `INSERT INTO push_subscriptions (id, profile_id, endpoint, p256dh, auth) VALUES ($1, $2, $3, 'key', 'auth')`,
		uuid.New(), coach, "https://push.example.com/"+uuid.NewString())
	rec = doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": true, "daily_cap": 3})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"on_duty":true`) || !strings.Contains(rec.Body.String(), `"daily_cap":3`) {
		t.Fatalf("on duty %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/coach/me", tc, nil)
	if !strings.Contains(rec.Body.String(), `"on_duty":true`) {
		t.Fatalf("me %s", rec.Body.String())
	}
}

// withCoachPhoto は、承認の前提になる顔写真を、応募中のコーチに直接つける。
func withCoachPhoto(t *testing.T, id uuid.UUID) {
	t.Helper()
	execSQLArgs(t, `UPDATE coach_profiles SET photo_public_id = $2 WHERE profile_id = $1`, id, "track-talk/"+uuid.NewString())
}

// dueAtNoonJST は、返却の目安が日本時間の12時か。DB から読んだ時刻はマシンのタイムゾーンで返るので、文字列ではなく時刻で比べる。
func dueAtNoonJST(s string) bool {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return false
	}
	t = t.In(jst.Zone)
	return t.Hour() == 12 && t.Minute() == 0
}

// 通知を使えるときは、登録があっても実際に届かない宛先だけでは受付中にできない。
func TestCoachDutyNeedsReachableDevice(t *testing.T) {
	pushServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/gone") {
			w.WriteHeader(http.StatusGone)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer pushServer.Close()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.VAPIDPublicKey = pub
		cfg.VAPIDPrivateKey = priv
		cfg.VAPIDSubject = "mailto:test@example.com"
		cfg.PushHTTP = pushServer.Client()
	})
	coach := uuid.New()
	tc := token(t, coach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tc, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d", rec.Code)
	}
	withCoachPhoto(t, coach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+coach.String()+"/approve", adminToken(t, h), nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d", rec.Code)
	}
	subscribe := func(suffix string) {
		p256dh, auth := testPushKeys(t)
		if rec := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions", tc, map[string]any{
			"endpoint": pushServer.URL + suffix, "keys": map[string]string{"p256dh": p256dh, "auth": auth},
		}); rec.Code != http.StatusNoContent {
			t.Fatalf("subscribe %s %d %s", suffix, rec.Code, rec.Body.String())
		}
	}
	subscribe("/gone")
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": true}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unreachable device %d %s", rec.Code, rec.Body.String())
	}
	subscribe("/ok")
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("reachable device %d %s", rec.Code, rec.Body.String())
	}
}
