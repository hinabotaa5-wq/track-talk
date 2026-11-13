package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 自分の投稿にいいねが付いたら、コメントと同じく投稿者へベルとプッシュで知らせる(本体 #608)。
// 設定はコメントの種類(comment)に従い、新しくいいねが入ったときだけ1件ずつ送る。
func TestPostLikeNotifiesAuthor(t *testing.T) {
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
	liker := tokenWithProfile(t, uuid.New(), "いいねする人", true)
	other := tokenWithProfile(t, uuid.New(), "もう一人", true)
	for _, sub := range []struct{ tok, path string }{{author, "/author"}, {liker, "/liker"}} {
		p256dh, auth := testPushKeys(t)
		if rec := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions", sub.tok, map[string]any{
			"endpoint": pushServer.URL + sub.path,
			"keys":     map[string]string{"p256dh": p256dh, "auth": auth},
		}); rec.Code != http.StatusNoContent {
			t.Fatalf("subscribe %d", rec.Code)
		}
	}
	pushesTo := func(path string) int {
		mu.Lock()
		defer mu.Unlock()
		n := 0
		for _, p := range paths {
			if p == path {
				n++
			}
		}
		return n
	}
	like := func(tok, postID string) {
		t.Helper()
		if rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+postID+"/likes", tok, nil); rec.Code != http.StatusNoContent {
			t.Fatalf("like %d %s", rec.Code, rec.Body.String())
		}
	}
	unlike := func(tok, postID string) {
		t.Helper()
		if rec := doJSON(t, h, http.MethodDelete, "/v1/posts/"+postID+"/likes", tok, nil); rec.Code != http.StatusNoContent {
			t.Fatalf("unlike %d %s", rec.Code, rec.Body.String())
		}
	}
	likeNotes := func(tok, postID string) []noteItem {
		items, _ := listNotifications(t, h, tok)
		return notesFor(items, postID, "post_like")
	}
	setComment := func(inApp, push bool) {
		t.Helper()
		if rec := doJSON(t, h, http.MethodPut, "/v1/me/notification-settings/comment", author, map[string]any{"in_app": inApp, "push": push}); rec.Code != http.StatusOK {
			t.Fatalf("setting %d %s", rec.Code, rec.Body.String())
		}
	}

	postID := createTextPost(t, h, author, "いいね通知")

	// 他人の新しいいいね: ベル1件・プッシュ1件。
	like(liker, postID)
	if n, p := len(likeNotes(author, postID)), pushesTo("/author"); n != 1 || p != 1 {
		t.Fatalf("new like: bell %d push %d", n, p)
	}
	if notes := likeNotes(author, postID); notes[0].Read {
		t.Fatalf("new like note should be unread: %+v", notes[0])
	}

	// いいね済みでの再押しは通知しない。
	like(liker, postID)
	if n, p := len(likeNotes(author, postID)), pushesTo("/author"); n != 1 || p != 1 {
		t.Fatalf("repeat like: bell %d push %d", n, p)
	}

	// 取り消しは通知せず、既存の通知も残す。
	unlike(liker, postID)
	if n, p := len(likeNotes(author, postID)), pushesTo("/author"); n != 1 || p != 1 {
		t.Fatalf("unlike: bell %d push %d", n, p)
	}

	// 取り消したあとの再いいねは、新しいいいねなので通知する。集約はしない。
	like(liker, postID)
	if n, p := len(likeNotes(author, postID)), pushesTo("/author"); n != 2 || p != 2 {
		t.Fatalf("re-like: bell %d push %d", n, p)
	}

	// 自分の投稿への自分のいいねは通知しない。
	like(author, postID)
	if n, p := len(likeNotes(author, postID)), pushesTo("/author"); n != 2 || p != 2 {
		t.Fatalf("self like: bell %d push %d", n, p)
	}
	// いいねした人には何も届かない。
	if n, p := len(likeNotes(liker, postID)), pushesTo("/liker"); n != 0 || p != 0 {
		t.Fatalf("liker got bell %d push %d", n, p)
	}

	// コメントの設定でベルをオフ: ベルは増えず、プッシュは届く。
	setComment(false, true)
	like(other, postID)
	if n, p := len(likeNotes(author, postID)), pushesTo("/author"); n != 2 || p != 3 {
		t.Fatalf("bell off: bell %d push %d", n, p)
	}

	// コメントの設定でプッシュをオフ: ベルは増え、プッシュは届かない。
	setComment(true, false)
	unlike(other, postID)
	like(other, postID)
	if n, p := len(likeNotes(author, postID)), pushesTo("/author"); n != 3 || p != 3 {
		t.Fatalf("push off: bell %d push %d", n, p)
	}

	// 既存の種類(コメント)の通知も従来どおり作れる。
	postComment(t, h, liker, postID, "コメントも届く", nil)
	items, _ := listNotifications(t, h, author)
	if len(notesFor(items, postID, "comment")) != 1 {
		t.Fatalf("comment note missing: %+v", items)
	}
}
