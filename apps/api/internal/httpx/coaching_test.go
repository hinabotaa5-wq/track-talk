package httpx_test

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

type coachingMe struct {
	BirthMonth *string `json:"birth_month"`
	IsMinor    *bool   `json:"is_minor"`
	Guardian   *struct {
		Status          string `json:"status"`
		MonthlyLimitYen *int   `json:"monthly_limit_yen"`
		AllowPublish    bool   `json:"allow_publish"`
	} `json:"guardian"`
	PushDevices int `json:"push_devices"`
}

type guardianView struct {
	Status          string  `json:"status"`
	GuardianName    *string `json:"guardian_name"`
	Relationship    *string `json:"relationship"`
	MonthlyLimitYen *int    `json:"monthly_limit_yen"`
	AllowPublish    bool    `json:"allow_publish"`
	LimitOptions    []*int  `json:"limit_options"`
	DefaultLimitYen int     `json:"default_limit_yen"`
	Token           string  `json:"token"`
}

// setupCoaching は、オンラインコーチングを有効にしたサーバーを作る。
func setupCoaching(t *testing.T) http.Handler {
	return setupConfig(t, func(cfg *config.Config) { cfg.CoachingEnabled = true })
}

func minorBirthMonth() string {
	return time.Now().AddDate(-15, 0, 0).Format("2006-01")
}

// 提供を始めるまでは、コーチングの API はどれも無いものとして 404 を返す。
func TestCoachingRoutesAreHiddenUntilEnabled(t *testing.T) {
	h := setup(t)
	ta := token(t, uuid.New())
	routes := []struct {
		method, path, tok string
	}{
		{http.MethodGet, "/v1/push/public-key", ""},
		{http.MethodGet, "/v1/guardian/abc", ""},
		{http.MethodPost, "/v1/guardian/abc/consent", ""},
		{http.MethodPatch, "/v1/guardian/abc", ""},
		{http.MethodPost, "/v1/guardian/abc/decline", ""},
		{http.MethodPost, "/v1/guardian/abc/revoke", ""},
		{http.MethodGet, "/v1/me/coaching", ta},
		{http.MethodGet, "/v1/me/coaching", ""},
		{http.MethodPut, "/v1/me/birth-month", ta},
		{http.MethodPost, "/v1/me/guardian-link", ta},
		{http.MethodPost, "/v1/me/push-subscriptions", ta},
		{http.MethodDelete, "/v1/me/push-subscriptions", ta},
		{http.MethodPost, "/v1/me/push-subscriptions/test", ta},
		{http.MethodPost, "/v1/uploads/coaching-video", ta},
	}
	for _, rt := range routes {
		if rec := doJSON(t, h, rt.method, rt.path, rt.tok, map[string]any{}); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s (token=%v) = %d %s", rt.method, rt.path, rt.tok != "", rec.Code, rec.Body.String())
		}
	}
	// ほかの API には影響しない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", ta, nil); rec.Code != http.StatusOK {
		t.Fatalf("me %d %s", rec.Code, rec.Body.String())
	}
}

func TestBirthMonthIsSetOnce(t *testing.T) {
	h := setupCoaching(t)
	a := uuid.New()
	ta := token(t, a)

	rec := doJSON(t, h, http.MethodGet, "/v1/me/coaching", ta, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("me %d %s", rec.Code, rec.Body.String())
	}
	var me coachingMe
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.BirthMonth != nil || me.IsMinor != nil || me.Guardian != nil {
		t.Fatalf("fresh %+v", me)
	}

	rec = doJSON(t, h, http.MethodPut, "/v1/me/birth-month", ta, map[string]any{"birth_month": "2999-01"})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "birth_month") {
		t.Fatalf("future %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPut, "/v1/me/birth-month", ta, map[string]any{"birth_month": minorBirthMonth()})
	if rec.Code != http.StatusOK {
		t.Fatalf("set %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.BirthMonth == nil || *me.BirthMonth != minorBirthMonth() || me.IsMinor == nil || !*me.IsMinor {
		t.Fatalf("after set %+v", me)
	}
	// 18歳未満が大人に書き換えられないよう、2回目は受け付けない。
	rec = doJSON(t, h, http.MethodPut, "/v1/me/birth-month", ta, map[string]any{"birth_month": "1990-01"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("second %d %s", rec.Code, rec.Body.String())
	}

	b := uuid.New()
	tb := token(t, b)
	rec = doJSON(t, h, http.MethodPut, "/v1/me/birth-month", tb, map[string]any{"birth_month": "1990-01"})
	if rec.Code != http.StatusOK {
		t.Fatalf("adult %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.IsMinor == nil || *me.IsMinor {
		t.Fatalf("adult %+v", me)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/me/guardian-link", tb, nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("adult link %d %s", rec.Code, rec.Body.String())
	}
}

func TestGuardianConsentFlow(t *testing.T) {
	h := setupCoaching(t)
	minor := uuid.New()
	tm := token(t, minor)

	rec := doJSON(t, h, http.MethodPost, "/v1/me/guardian-link", tm, nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("link before birth month %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/birth-month", tm, map[string]any{"birth_month": minorBirthMonth()}); rec.Code != http.StatusOK {
		t.Fatalf("birth month %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/me/guardian-link", tm, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("link %d %s", rec.Code, rec.Body.String())
	}
	var issued guardianView
	_ = json.Unmarshal(rec.Body.Bytes(), &issued)
	if issued.Status != "pending" || len(issued.Token) != 43 {
		t.Fatalf("issued %+v", issued)
	}
	path := "/v1/guardian/" + issued.Token

	// 保護者のページはログインなしで開ける。子のアカウントの情報は返さない。
	rec = doJSON(t, h, http.MethodGet, path, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), minor.String()) {
		t.Fatal("guardian page leaks the minor's id")
	}
	var view guardianView
	_ = json.Unmarshal(rec.Body.Bytes(), &view)
	if view.Status != "pending" || view.DefaultLimitYen != 3000 || len(view.LimitOptions) != 4 || view.LimitOptions[3] != nil {
		t.Fatalf("view %+v", view)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/guardian/"+strings.Repeat("a", 43), "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown token %d", rec.Code)
	}

	// 同意の前に上限は変えられない。
	if rec := doJSON(t, h, http.MethodPatch, path, "", map[string]any{"monthly_limit_yen": 5000}); rec.Code != http.StatusConflict {
		t.Fatalf("patch before consent %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, path+"/consent", "", map[string]any{
		"guardian_name": "", "relationship": "uncle", "monthly_limit_yen": 2000, "agree": false,
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid consent %d %s", rec.Code, rec.Body.String())
	}
	for _, field := range []string{"guardian_name", "relationship", "monthly_limit_yen", "agree"} {
		if !strings.Contains(rec.Body.String(), field) {
			t.Errorf("missing field error %s: %s", field, rec.Body.String())
		}
	}

	// 上限を選ばなければ3,000円になる。
	rec = doJSON(t, h, http.MethodPost, path+"/consent", "", map[string]any{
		"guardian_name": "山田 花子", "relationship": "mother", "agree": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("consent %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &view)
	if view.Status != "consented" || view.MonthlyLimitYen == nil || *view.MonthlyLimitYen != 3000 || view.AllowPublish {
		t.Fatalf("consented %+v", view)
	}

	// null は上限なし。
	rec = doJSON(t, h, http.MethodPatch, path, "", map[string]any{"monthly_limit_yen": nil, "allow_publish": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &view)
	if view.MonthlyLimitYen != nil || !view.AllowPublish {
		t.Fatalf("patched %+v", view)
	}
	if rec := doJSON(t, h, http.MethodPatch, path, "", map[string]any{"monthly_limit_yen": 4000}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("patch invalid limit %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/me/coaching", tm, nil)
	var me coachingMe
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Guardian == nil || me.Guardian.Status != "consented" || me.Guardian.MonthlyLimitYen != nil || !me.Guardian.AllowPublish {
		t.Fatalf("minor view %+v", me)
	}

	// 同意したあとは「同意しない」ではなく「取り消す」。
	if rec := doJSON(t, h, http.MethodPost, path+"/decline", "", nil); rec.Code != http.StatusConflict {
		t.Fatalf("decline after consent %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, path+"/revoke", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &view)
	if view.Status != "revoked" {
		t.Fatalf("revoked %+v", view)
	}

	// リンクを出し直すと、前のリンクは使えなくなる。状態は引き継ぐ。
	rec = doJSON(t, h, http.MethodPost, "/v1/me/guardian-link", tm, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("reissue %d %s", rec.Code, rec.Body.String())
	}
	var reissued guardianView
	_ = json.Unmarshal(rec.Body.Bytes(), &reissued)
	if reissued.Token == issued.Token || reissued.Status != "revoked" {
		t.Fatalf("reissued %+v", reissued)
	}
	if rec := doJSON(t, h, http.MethodGet, path, "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("old link %d", rec.Code)
	}
}

func TestGuardianCanDeclineBeforeConsenting(t *testing.T) {
	h := setupCoaching(t)
	minor := uuid.New()
	tm := token(t, minor)
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/birth-month", tm, map[string]any{"birth_month": minorBirthMonth()}); rec.Code != http.StatusOK {
		t.Fatalf("birth month %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/me/guardian-link", tm, nil)
	var issued guardianView
	_ = json.Unmarshal(rec.Body.Bytes(), &issued)
	path := "/v1/guardian/" + issued.Token

	if rec := doJSON(t, h, http.MethodPost, path+"/revoke", "", nil); rec.Code != http.StatusConflict {
		t.Fatalf("revoke before consent %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, path+"/decline", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("decline %d %s", rec.Code, rec.Body.String())
	}
	var view guardianView
	_ = json.Unmarshal(rec.Body.Bytes(), &view)
	if view.Status != "declined" {
		t.Fatalf("declined %+v", view)
	}
	// 断ったあとで考え直して同意することはできる。
	rec = doJSON(t, h, http.MethodPost, path+"/consent", "", map[string]any{
		"guardian_name": "山田 太郎", "relationship": "father", "monthly_limit_yen": 1000, "agree": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("consent after decline %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &view)
	if view.Status != "consented" || view.MonthlyLimitYen == nil || *view.MonthlyLimitYen != 1000 {
		t.Fatalf("consented %+v", view)
	}
}

func testPushKeys(t *testing.T) (p256dh, auth string) {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 16)
	_, _ = rand.Read(secret)
	return base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()), base64.RawURLEncoding.EncodeToString(secret)
}

func TestPushSubscriptionsAndTestNotification(t *testing.T) {
	var gone atomic.Bool
	var received atomic.Int64
	pushServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/gone") && gone.Load() {
			w.WriteHeader(http.StatusGone)
			return
		}
		received.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer pushServer.Close()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}

	unconfigured := setupCoaching(t)
	if rec := doJSON(t, unconfigured, http.MethodGet, "/v1/push/public-key", "", nil); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured key %d", rec.Code)
	}

	h := setupConfig(t, func(cfg *config.Config) {
		cfg.CoachingEnabled = true
		cfg.VAPIDPublicKey = pub
		cfg.VAPIDPrivateKey = priv
		cfg.VAPIDSubject = "mailto:test@example.com"
		cfg.PushHTTP = pushServer.Client()
	})

	rec := doJSON(t, h, http.MethodGet, "/v1/push/public-key", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), pub) {
		t.Fatalf("public key %d %s", rec.Code, rec.Body.String())
	}

	a := uuid.New()
	ta := token(t, a)
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions/test", ta, nil); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("test without device %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions", ta, map[string]any{
		"endpoint": "http://insecure.example.com/x", "keys": map[string]string{"p256dh": "a", "auth": "b"},
	}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("insecure endpoint %d", rec.Code)
	}

	for _, suffix := range []string{"/ok", "/gone"} {
		p256dh, auth := testPushKeys(t)
		rec := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions", ta, map[string]any{
			"endpoint":       pushServer.URL + suffix,
			"expirationTime": nil,
			"keys":           map[string]string{"p256dh": p256dh, "auth": auth},
		})
		if rec.Code != http.StatusNoContent {
			t.Fatalf("subscribe %s %d %s", suffix, rec.Code, rec.Body.String())
		}
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/coaching", ta, nil)
	var me coachingMe
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.PushDevices != 2 {
		t.Fatalf("devices %+v", me)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions/test", ta, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"sent":2`) {
		t.Fatalf("test %d %s", rec.Code, rec.Body.String())
	}

	// 使えなくなった宛先は送るときに消える。
	gone.Store(true)
	rec = doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions/test", ta, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"sent":1`) {
		t.Fatalf("test with gone %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/coaching", ta, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.PushDevices != 1 {
		t.Fatalf("devices after gone %+v", me)
	}

	// 他人の端末は消せない。
	b := uuid.New()
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me/push-subscriptions", token(t, b), map[string]any{"endpoint": pushServer.URL + "/ok"}); rec.Code != http.StatusNoContent {
		t.Fatalf("delete other %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/coaching", ta, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.PushDevices != 1 {
		t.Fatalf("other user removed a device %+v", me)
	}
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me/push-subscriptions", ta, map[string]any{"endpoint": pushServer.URL + "/ok"}); rec.Code != http.StatusNoContent {
		t.Fatalf("delete %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/coaching", ta, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.PushDevices != 0 {
		t.Fatalf("devices after delete %+v", me)
	}
}

func TestCoachingVideoUploadIsPrivate(t *testing.T) {
	t.Setenv("CLOUDINARY_CLOUD_NAME", "demo")
	t.Setenv("CLOUDINARY_API_KEY", "key")
	t.Setenv("CLOUDINARY_API_SECRET", "secret")
	h := setupCoaching(t)
	if rec := doJSON(t, h, http.MethodPost, "/v1/uploads/coaching-video", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous %d", rec.Code)
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/uploads/coaching-video", token(t, uuid.New()), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("sign %d %s", rec.Code, rec.Body.String())
	}
	var signed struct {
		PublicID string `json:"public_id"`
		Type     string `json:"type"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &signed)
	if signed.Type != "authenticated" || !strings.HasPrefix(signed.PublicID, "track-talk-coaching/") {
		t.Fatalf("signed %+v", signed)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatal("secret leaked")
	}
}

func TestGuardianConsentNotifiesTheMinor(t *testing.T) {
	var received atomic.Int64
	pushServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer pushServer.Close()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.CoachingEnabled = true
		cfg.VAPIDPublicKey = pub
		cfg.VAPIDPrivateKey = priv
		cfg.VAPIDSubject = "mailto:test@example.com"
		cfg.PushHTTP = pushServer.Client()
	})

	minor := uuid.New()
	tm := token(t, minor)
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/birth-month", tm, map[string]any{"birth_month": minorBirthMonth()}); rec.Code != http.StatusOK {
		t.Fatalf("birth month %d %s", rec.Code, rec.Body.String())
	}
	p256dh, auth := testPushKeys(t)
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions", tm, map[string]any{
		"endpoint": pushServer.URL + "/minor",
		"keys":     map[string]string{"p256dh": p256dh, "auth": auth},
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("subscribe %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/me/guardian-link", tm, nil)
	var issued guardianView
	_ = json.Unmarshal(rec.Body.Bytes(), &issued)
	path := "/v1/guardian/" + issued.Token
	body := map[string]any{"guardian_name": "山田 花子", "relationship": "mother", "agree": true}

	if rec := doJSON(t, h, http.MethodPost, path+"/consent", "", body); rec.Code != http.StatusOK {
		t.Fatalf("consent %d %s", rec.Code, rec.Body.String())
	}
	if got := received.Load(); got != 1 {
		t.Fatalf("pushes after consent %d", got)
	}
	// 同意済みのまま送り直しても、もう一度は知らせない。
	if rec := doJSON(t, h, http.MethodPost, path+"/consent", "", body); rec.Code != http.StatusOK {
		t.Fatalf("consent again %d %s", rec.Code, rec.Body.String())
	}
	if got := received.Load(); got != 1 {
		t.Fatalf("pushes after second consent %d", got)
	}
}
