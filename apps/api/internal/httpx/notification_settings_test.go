package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

type notificationSetting struct {
	Category string `json:"category"`
	InApp    bool   `json:"in_app"`
	Push     bool   `json:"push"`
}

func notificationSettings(t *testing.T, h http.Handler, tok string) map[string]notificationSetting {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/me/notification-settings", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("settings %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Settings []notificationSetting `json:"settings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	out := map[string]notificationSetting{}
	for _, s := range body.Settings {
		out[s.Category] = s
	}
	return out
}

// 通知の種類ごとに、ベルとプッシュを別々に切り替えられる。募集のプッシュだけは、初めはオフ。
func TestNotificationSettingsDefaultsAndUpdate(t *testing.T) {
	h := setup(t)
	tok := token(t, uuid.New())

	got := notificationSettings(t, h, tok)
	if len(got) != 8 {
		t.Fatalf("categories %+v", got)
	}
	for key, s := range got {
		wantPush := key != "recruitment"
		if !s.InApp || s.Push != wantPush {
			t.Fatalf("default %s %+v", key, s)
		}
	}

	if rec := doJSON(t, h, http.MethodPut, "/v1/me/notification-settings/comment", tok, map[string]any{"in_app": false, "push": true}); rec.Code != http.StatusOK {
		t.Fatalf("update %d %s", rec.Code, rec.Body.String())
	}
	if s := notificationSettings(t, h, tok)["comment"]; s.InApp || !s.Push {
		t.Fatalf("after update %+v", s)
	}
	// 片方だけ送ると、もう片方が知らないうちに変わるので受け付けない。
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/notification-settings/comment", tok, map[string]any{"push": false}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("partial %d", rec.Code)
	}
	// お金や保護者の同意などの大事な知らせは切り替えられない。
	for _, key := range []string{"important", "nope"} {
		if rec := doJSON(t, h, http.MethodPut, "/v1/me/notification-settings/"+key, tok, map[string]any{"in_app": false, "push": false}); rec.Code != http.StatusNotFound {
			t.Fatalf("%s %d", key, rec.Code)
		}
	}
	// ほかの人の設定は変わらない。
	if s := notificationSettings(t, h, token(t, uuid.New()))["comment"]; !s.InApp || !s.Push {
		t.Fatalf("other user %+v", s)
	}
}

// コメントの知らせは、ベルとプッシュの両方で届き、本人がオフにした方には届かない。
func TestCommentNotificationFollowsSettings(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	pushServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
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
		cfg.CoachingEnabled = false
	})
	author := tokenWithProfile(t, uuid.New(), "投稿主", true)
	other := tokenWithProfile(t, uuid.New(), "コメントする人", true)
	p256dh, auth := testPushKeys(t)
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions", author, map[string]any{
		"endpoint": pushServer.URL + "/author",
		"keys":     map[string]string{"p256dh": p256dh, "auth": auth},
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("subscribe %d", rec.Code)
	}
	pushes := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(paths)
	}
	setComment := func(inApp, push bool) {
		t.Helper()
		if rec := doJSON(t, h, http.MethodPut, "/v1/me/notification-settings/comment", author, map[string]any{"in_app": inApp, "push": push}); rec.Code != http.StatusOK {
			t.Fatalf("setting %d %s", rec.Code, rec.Body.String())
		}
	}
	postID := createTextPost(t, h, author, "設定のテスト")
	commentNotes := func() int {
		items, _ := listNotifications(t, h, author)
		return len(notesFor(items, postID, "comment"))
	}

	postComment(t, h, other, postID, "両方オン", nil)
	if commentNotes() != 1 || pushes() != 1 {
		t.Fatalf("both on: bell %d push %d", commentNotes(), pushes())
	}

	setComment(false, true)
	postComment(t, h, other, postID, "ベルはオフ", nil)
	if commentNotes() != 1 || pushes() != 2 {
		t.Fatalf("bell off: bell %d push %d", commentNotes(), pushes())
	}

	setComment(true, false)
	postComment(t, h, other, postID, "プッシュはオフ", nil)
	if commentNotes() != 2 || pushes() != 2 {
		t.Fatalf("push off: bell %d push %d", commentNotes(), pushes())
	}

	// ほかの種類の設定は、コメントの設定に引きずられない。
	if s := notificationSettings(t, h, author)["practice"]; !s.InApp || !s.Push {
		t.Fatalf("practice %+v", s)
	}
}
