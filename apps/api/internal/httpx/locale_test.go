package httpx

import (
	"testing"

	"github.com/hinata/track-talk/apps/api/internal/push"
)

// 表示言語の正規化と通知文面の言語追従（本体 #521）。未訳は日本語フォールバック。

func TestNormalizeLocale(t *testing.T) {
	for in, want := range map[string]string{
		"en": "en",
		"ja": "ja",
		"":   "ja",
		"EN": "ja",
		"fr": "ja",
	} {
		if got := normalizeLocale(in); got != want {
			t.Fatalf("normalizeLocale(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLocalizePushMessageEnglish(t *testing.T) {
	got := localizePushMessage("en", push.Message{
		Title: "フォーム添削の依頼が届きました",
		Body:  "返答が届くまでお待ちください",
		URL:   "/coach",
		Tag:   "review-1",
	})
	if got.Title != "New form-review request" {
		t.Fatalf("title = %q", got.Title)
	}
	if got.Body != "Please wait for the response" {
		t.Fatalf("body = %q", got.Body)
	}
	if got.URL != "/coach" || got.Tag != "review-1" {
		t.Fatalf("url/tag changed: %+v", got)
	}
}

func TestLocalizePushMessageFallback(t *testing.T) {
	// ja はそのまま。
	ja := push.Message{Title: "フォーム添削の依頼が届きました", Body: "返答が届くまでお待ちください"}
	if got := localizePushMessage("ja", ja); got != ja {
		t.Fatalf("ja changed: %+v", got)
	}
	// 未知の言語設定も ja。
	if got := localizePushMessage("fr", ja); got != ja {
		t.Fatalf("unknown locale changed: %+v", got)
	}
	// en でも表にない本文（名前・金額・日時入り）は日本語のまま。表題だけ置き換える。
	dynamic := push.Message{Title: "パーソナル指導の期間が終わりました", Body: "「大会プラン」の期間が終わりました。やりとりは見られなくなりました"}
	got := localizePushMessage("en", dynamic)
	if got.Title != "Personal coaching ended" {
		t.Fatalf("title = %q", got.Title)
	}
	if got.Body != dynamic.Body {
		t.Fatalf("dynamic body must stay Japanese, got %q", got.Body)
	}
	// 表にない表題はそのまま（空白やキー名の露出なし）。
	unknown := push.Message{Title: "お知らせがあります", Body: "本文です"}
	if got := localizePushMessage("en", unknown); got != unknown {
		t.Fatalf("unknown message changed: %+v", got)
	}
}
