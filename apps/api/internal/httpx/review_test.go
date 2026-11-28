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
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
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

// grantUpload は、id をアップロード署名で tok のユーザーに発行したことにする。
// 署名の API を通さないのは、書き込みの回数制限を使わないため。持ち主は入れ替えられる。
func grantUpload(t *testing.T, h http.Handler, tok, id string) {
	t.Helper()
	// プロフィールは最初の API 呼び出しで作られる。
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tok, nil); rec.Code != http.StatusOK {
		t.Fatalf("me %d %s", rec.Code, rec.Body.String())
	}
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(tok, claims); err != nil {
		t.Fatal(err)
	}
	execSQLArgs(t, `INSERT INTO cloudinary_uploads (public_id, owner_id) VALUES ($1, $2::uuid)
		ON CONFLICT (public_id) DO UPDATE SET owner_id = EXCLUDED.owner_id`, id, claims["sub"])
}

// signedCoachingVideo は、tok のユーザーに発行された添削用の動画 ID を返す。
func signedCoachingVideo(t *testing.T, h http.Handler, tok string) string {
	t.Helper()
	id := "track-talk-coaching/" + uuid.NewString()
	grantUpload(t, h, tok, id)
	return id
}

func reviewBodyFor(t *testing.T, h http.Handler, tok string) map[string]any {
	t.Helper()
	return map[string]any{
		"guardian_permission": true,
		"tier":                "coach",
		"event":               "100m",
		"focus":               []string{"rhythm", "start", "start"},
		"video_public_id":     signedCoachingVideo(t, h, tok),
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
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBodyFor(t, h, tu))
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

	// 生まれた年月がなくても送れる。
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBodyFor(t, h, tu))
	if rec.Code != http.StatusCreated {
		t.Fatalf("no birth month %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/birth-month", tu, map[string]any{"birth_month": "1995-04"}); rec.Code != http.StatusOK {
		t.Fatalf("birth month %d", rec.Code)
	}

	bad := reviewBodyFor(t, h, tu)
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

	body := reviewBodyFor(t, h, tu)
	sentAt := time.Now()
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created reviewRequestView
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Status != "queued" || strings.Join(created.Focus, ",") != "start,rhythm" || !dueAtIn24h(created.DueAt, sentAt, time.Now()) || !strings.Contains(created.VideoURL, "/video/authenticated/s--") {
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
	// 一覧の各行は、一覧ページが使う項目だけ（本体 #1072 の6項目＋プレミアムの印の plan、本体 #1053）。meta は従来どおり。
	var list struct {
		Requests []map[string]json.RawMessage `json:"requests"`
		Meta     struct {
			Page    int   `json:"page"`
			PerPage int   `json:"per_page"`
			Total   int64 `json:"total"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	// このテストでは上で2件作っている（デモコード付きの1件と created）。新しい順で created が先頭。
	if len(list.Requests) != 2 || list.Meta.Page != 1 || list.Meta.PerPage != 20 || list.Meta.Total != 2 {
		t.Fatalf("list meta %s", rec.Body.String())
	}
	wantKeys := []string{"id", "plan", "event", "status", "canceled_at", "withdrawal", "created_at"}
	for _, it := range list.Requests {
		if len(it) != len(wantKeys) {
			t.Fatalf("list item keys %s", rec.Body.String())
		}
		for _, k := range wantKeys {
			if _, ok := it[k]; !ok {
				t.Fatalf("list item missing %s: %s", k, rec.Body.String())
			}
		}
	}
	item := list.Requests[0]
	if string(item["id"]) != `"`+created.ID+`"` || string(item["status"]) != `"queued"` || string(item["canceled_at"]) != "null" || string(item["withdrawal"]) != "null" {
		t.Fatalf("list item %s", rec.Body.String())
	}
	// 詳細は従来どおり全項目を返す。
	rec = doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+created.ID, tu, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}
	for _, k := range []string{"tier", "focus", "question", "due_at", "video_url", "thumbnail_url", "cash_price_yen", "info_request", "videos_purged", "question_en"} {
		if !strings.Contains(rec.Body.String(), `"`+k+`":`) {
			t.Fatalf("detail missing %s: %s", k, rec.Body.String())
		}
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

// dueAtIn24h は、返却期限が送った時刻（before〜after のあいだ）の24時間後か。DB から読んだ時刻はマシンのタイムゾーンで返るので、文字列ではなく時刻で比べる。
func dueAtIn24h(s string, before, after time.Time) bool {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return false
	}
	return dueIn24h(t, before, after)
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

// 受付停止中のコーチには新しい依頼が届かず（選べる一覧からも消える）、受付済みの依頼はそのまま進められる。
// 受付を再開すると、また依頼が届く。状態はサーバーに残る（Issue #320）。
func TestCoachDutyToggleStopsNewRequestsOnly(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	nominated := func() bool {
		rec := doJSON(t, h, http.MethodGet, "/v1/coaching/nominees?event=100m", tu, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("nominees %d %s", rec.Code, rec.Body.String())
		}
		return strings.Contains(rec.Body.String(), coach.String())
	}
	id := chosenReview(t, h, tu, coach.String(), "card", false)
	payChosenReview(t, h, id, "card")
	offers := coachRequests(t, h, tc).Reviewing
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+offers[0].ID+"/start", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("accept %d", rec.Code)
	}

	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": false}); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"on_duty":false`) {
		t.Fatalf("off %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/coach/me", tc, nil); !strings.Contains(rec.Body.String(), `"on_duty":false`) {
		t.Fatalf("not persisted %s", rec.Body.String())
	}
	if nominated() {
		t.Fatal("off-duty coach is still offered to requesters")
	}
	b := reviewBodyFor(t, h, tu)
	b["method"], b["requested_coach_id"] = "card", coach.String()
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, b); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "requested_coach_id") {
		t.Fatalf("request to off-duty coach %d %s", rec.Code, rec.Body.String())
	}
	// 受付済みの依頼は止まらない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, responseBody()); rec.Code != http.StatusOK {
		t.Fatalf("response while off duty %d %s", rec.Code, rec.Body.String())
	}

	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("on %d %s", rec.Code, rec.Body.String())
	}
	if !nominated() {
		t.Fatal("coach not offered after resuming")
	}
	next := chosenReview(t, h, tu, coach.String(), "card", false)
	payChosenReview(t, h, next, "card")
	if offers := coachRequests(t, h, tc).Reviewing; len(offers) != 1 || offers[0].ID != next {
		t.Fatalf("offers after resuming %+v", offers)
	}
}

// TestReviewRequestAthleteInfo は、申込の任意4項目（本体 #1013 補足：PB・目標タイム・風速・その時の体のコンディション）と、
// 5項目目の「意識していること」（focus_note、本体 #1093 の A1〜A3）を確かめる。
func TestReviewRequestAthleteInfo(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, tadmin)
	type infoView struct {
		ID            string  `json:"id"`
		PB            *string `json:"pb"`
		GoalTime      *string `json:"goal_time"`
		WindSpeed     *string `json:"wind_speed"`
		BodyCondition *string `json:"body_condition"`
		FocusNote     *string `json:"focus_note"`
	}
	info := map[string]any{
		"focus_note":     "接地で地面を押すこと",
		"pb":             "11秒20（2025年）",
		"goal_time":      "10秒台",
		"wind_speed":     "+1.2",
		"body_condition": "前日に脚が張っていた",
	}
	create := func(tu string, body map[string]any) *httptest.ResponseRecorder {
		t.Helper()
		return doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body)
	}
	assertInfo := func(label, body string, want map[string]any) {
		t.Helper()
		var got infoView
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Fatalf("%s json %v %s", label, err, body)
		}
		for key, ptr := range map[string]*string{"pb": got.PB, "goal_time": got.GoalTime, "wind_speed": got.WindSpeed, "body_condition": got.BodyCondition, "focus_note": got.FocusNote} {
			w, ok := want[key].(string)
			if !ok {
				if ptr != nil || !strings.Contains(body, `"`+key+`":null`) {
					t.Fatalf("%s %s want null: %s", label, key, body)
				}
				continue
			}
			if ptr == nil || *ptr != w {
				t.Fatalf("%s %s want %q: %s", label, key, w, body)
			}
		}
		// 本補足で作らないことにした項目は、応答のキーにも出さない。
		for _, k := range []string{`"goal_meet"`, `"filming_context"`} {
			if strings.Contains(body, k) {
				t.Fatalf("%s has %s: %s", label, k, body)
			}
		}
	}

	// A6: 上限超過・連絡先らしき文字列は、その項目の名前つきで validation_error。上限ちょうどは通る長さ。
	_, tv := demoRequester(t, h, tadmin)
	limits := map[string]int{"pb": 100, "goal_time": 100, "wind_speed": 50, "body_condition": 300, "focus_note": 300}
	for key, limit := range limits {
		body := reviewBodyFor(t, h, tv)
		body[key] = strings.Repeat("あ", limit+1)
		if rec := create(tv, body); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"validation_error"`) || !strings.Contains(rec.Body.String(), `"`+key+`"`) {
			t.Fatalf("A6 long %s: %d %s", key, rec.Code, rec.Body.String())
		}
		body = reviewBodyFor(t, h, tv)
		body[key] = "090-1234-5678"
		if rec := create(tv, body); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"`+key+`"`) {
			t.Fatalf("A6 contact %s: %d %s", key, rec.Code, rec.Body.String())
		}
	}
	atLimit := map[string]any{}
	for key, limit := range limits {
		atLimit[key] = strings.Repeat("あ", limit)
	}
	body := reviewBodyFor(t, h, tv)
	for k, v := range atLimit {
		body[k] = v
	}
	rec := create(tv, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("A6 at limit %d %s", rec.Code, rec.Body.String())
	}
	assertInfo("A6 at limit", rec.Body.String(), atLimit)

	// A5: 相談内容のある経路で4項目を送ると保存され、申込応答とコーチ側 GET に入る。
	_, tu := demoRequester(t, h, tadmin)
	body = reviewBodyFor(t, h, tu)
	delete(body, "focus")
	body["question"] = "スタートの出だしで腰が落ちる気がします"
	for k, v := range info {
		body[k] = v
	}
	rec = create(tu, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("A5 create %d %s", rec.Code, rec.Body.String())
	}
	assertInfo("A5 create", rec.Body.String(), info)
	var created infoView
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	assertInfo("A5 requester get", doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+created.ID, tu, nil).Body.String(), info)
	coachGet := func(label, id string, want map[string]any) {
		t.Helper()
		for _, o := range coachRequests(t, h, tc).Reviewing {
			if o.ID == id {
				if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/start", tc, nil); rec.Code != http.StatusOK {
					t.Fatalf("%s start %d %s", label, rec.Code, rec.Body.String())
				}
			}
		}
		rec := doJSON(t, h, http.MethodGet, "/v1/coach/requests/"+id, tc, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s coach get %d %s", label, rec.Code, rec.Body.String())
		}
		assertInfo(label+" coach get", rec.Body.String(), want)
	}
	coachGet("A5", created.ID, info)

	// A5: 送らない・空白だけのときは null。
	_, tn := demoRequester(t, h, tadmin)
	body = reviewBodyFor(t, h, tn)
	body["wind_speed"] = "   "
	rec = create(tn, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("A5 none %d %s", rec.Code, rec.Body.String())
	}
	assertInfo("A5 none", rec.Body.String(), map[string]any{})
	var none infoView
	_ = json.Unmarshal(rec.Body.Bytes(), &none)
	coachGet("A5 none", none.ID, map[string]any{})

	// A7: 動画のみ経路（question なし・見てほしい点だけ）でも4項目が保存される。
	_, to := demoRequester(t, h, tadmin)
	body = reviewBodyFor(t, h, to)
	for k, v := range info {
		body[k] = v
	}
	rec = create(to, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("A7 create %d %s", rec.Code, rec.Body.String())
	}
	assertInfo("A7 create", rec.Body.String(), info)
	var videoOnly infoView
	_ = json.Unmarshal(rec.Body.Bytes(), &videoOnly)
	var stored [5]*string
	queryRowArgs(t, `SELECT pb, goal_time, wind_speed, body_condition, focus_note FROM review_requests WHERE id = $1`, []any{videoOnly.ID}, &stored[0], &stored[1], &stored[2], &stored[3], &stored[4])
	for i, want := range []string{"11秒20（2025年）", "10秒台", "+1.2", "前日に脚が張っていた", "接地で地面を押すこと"} {
		if stored[i] == nil || *stored[i] != want {
			t.Fatalf("A7 stored[%d] %v", i, stored[i])
		}
	}
}
