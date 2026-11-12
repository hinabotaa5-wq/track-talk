package httpx_test

// プッシュのペイロードに、その通知を送る直前の受け手の未読総数が入る。
// ベルの未読数と同じ数え方で、照合を通過した端末だけに届く。DB の変更は要らない。

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"golang.org/x/crypto/hkdf"

	"github.com/hinata/track-talk/apps/api/internal/config"
)

// capturePushes は、送られてきたプッシュの暗号文を宛先ごとに残すテスト用サーバー。
type capturePushes struct {
	mu     sync.Mutex
	bodies map[string][][]byte
	server *httptest.Server
}

func newCapturePushes(t *testing.T) *capturePushes {
	t.Helper()
	c := &capturePushes{bodies: map[string][][]byte{}}
	c.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		c.mu.Lock()
		c.bodies[r.URL.Path] = append(c.bodies[r.URL.Path], body)
		c.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(c.server.Close)
	return c
}

func (c *capturePushes) get(path string) [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([][]byte{}, c.bodies[path]...)
}

// decryptPush は、Web Push（RFC 8291、aes128gcm）の暗号文を購読者の鍵で開く。
// webpush-go が送る単一レコードだけを扱う。
func decryptPush(t *testing.T, body []byte, subPriv *ecdh.PrivateKey, authSecret []byte) map[string]any {
	t.Helper()
	if len(body) < 21 {
		t.Fatalf("push body too short %d", len(body))
	}
	salt := body[:16]
	idLen := int(body[20])
	if idLen != 65 || len(body) < 21+idLen+16 {
		t.Fatalf("unexpected sender key length %d (body %d)", idLen, len(body))
	}
	senderRaw := body[21 : 21+idLen]
	ciphertext := body[21+idLen:]
	senderPub, err := ecdh.P256().NewPublicKey(senderRaw)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := subPriv.ECDH(senderPub)
	if err != nil {
		t.Fatal(err)
	}
	extract := func(secret, salt []byte) []byte {
		return hkdf.Extract(sha256.New, secret, salt)
	}
	expand := func(prk, info []byte, length int) []byte {
		out := make([]byte, length)
		if _, err := io.ReadFull(hkdf.Expand(sha256.New, prk, info), out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	subRaw := subPriv.PublicKey().Bytes()
	if len(subRaw) != 65 {
		t.Fatalf("unexpected subscriber key length %d", len(subRaw))
	}
	keyInfo := append(append([]byte("WebPush: info\x00"), subRaw...), senderRaw...)
	ikm := expand(extract(shared, authSecret), keyInfo, 32)
	prk := extract(ikm, salt)
	cek := expand(prk, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := expand(prk, []byte("Content-Encoding: nonce\x00"), 12)
	block, err := aes.NewCipher(cek)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		t.Fatal(err)
	}
	// webpush-go はレコード長まで 0x00 で埋める。末尾は区切り 0x02。
	plain = bytes.TrimRight(plain, "\x00")
	if len(plain) == 0 || plain[len(plain)-1] != 0x02 {
		t.Fatalf("unexpected padding %x", plain)
	}
	plain = plain[:len(plain)-1]
	var msg map[string]any
	if err := json.Unmarshal(plain, &msg); err != nil {
		t.Fatalf("payload %q: %v", plain, err)
	}
	return msg
}

func TestAnnouncementPushCarriesUnreadCount(t *testing.T) {
	captures := newCapturePushes(t)
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.VAPIDPublicKey = pub
		cfg.VAPIDPrivateKey = priv
		cfg.VAPIDSubject = "mailto:test@example.com"
		cfg.PushHTTP = captures.server.Client()
	})

	// 購読の鍵は自分で作り、秘密鍵を残して復号に使う。
	subPriv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authSecret := make([]byte, 16)
	if _, err := rand.Read(authSecret); err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{
		"p256dh": base64.RawURLEncoding.EncodeToString(subPriv.PublicKey().Bytes()),
		"auth":   base64.RawURLEncoding.EncodeToString(authSecret),
	}

	admin := adminToken(t, h)
	a := token(t, uuid.New())
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions", a, map[string]any{
		"endpoint": captures.server.URL + "/a", "keys": keys,
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("subscribe %d %s", rec.Code, rec.Body.String())
	}

	send := func(title, body string) {
		t.Helper()
		msg := map[string]any{"title": title, "body": body, "url": "/coaches"}
		if rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements", admin, withAnnouncement(msg, uuid.NewString(), "送信")); rec.Code != http.StatusOK {
			t.Fatalf("send %d %s", rec.Code, rec.Body.String())
		}
	}

	send("お知らせ1", "本文1")
	got := captures.get("/a")
	if len(got) != 1 {
		t.Fatalf("pushes %d", len(got))
	}
	first := decryptPush(t, got[0], subPriv, authSecret)
	if first["unread_count"] != float64(1) {
		t.Fatalf("unread_count %+v", first)
	}
	if first["subscription_id"] == nil || first["title"] != "お知らせ1" {
		t.Fatalf("payload %+v", first)
	}
	if items, unread := listNotifications(t, h, a); unread != 1 || len(items) != 1 {
		t.Fatalf("bell unread=%d items=%d", unread, len(items))
	}

	// 2通目の時点では未読が2件になり、送る直前の総数が入る（増分の推測ではない）。
	send("お知らせ2", "本文2")
	got = captures.get("/a")
	if len(got) != 2 {
		t.Fatalf("pushes %d", len(got))
	}
	second := decryptPush(t, got[1], subPriv, authSecret)
	if second["unread_count"] != float64(2) {
		t.Fatalf("unread_count %+v", second)
	}
	if _, unread := listNotifications(t, h, a); unread != 2 {
		t.Fatalf("bell unread=%d", unread)
	}
}
