package httpx

import (
	"testing"

	"github.com/google/uuid"

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
		Title: "動画添削の依頼が届きました",
		Body:  "返答が届くまでお待ちください",
		URL:   "/coach",
		Tag:   "review-1",
	})
	if got.Title != "New video-review request" {
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
	ja := push.Message{Title: "動画添削の依頼が届きました", Body: "返答が届くまでお待ちください"}
	if got := localizePushMessage("ja", ja); got != ja {
		t.Fatalf("ja changed: %+v", got)
	}
	// 未知の言語設定も ja。
	if got := localizePushMessage("fr", ja); got != ja {
		t.Fatalf("unknown locale changed: %+v", got)
	}
	// en でも表にない本文（名前・金額・日時入り）は日本語のまま。表題だけ置き換える。
	dynamic := push.Message{Title: "パーソナル指導の期間が終わりました", Body:  "「大会プラン」の期間が終わりました。やりとりは見られなくなりました"}
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

// 通知は表題1行だけにした（本体 #609）。短くした表題は英語にも置き換わり、空の本文は空のまま出す。
func TestLocalizeShortenedNotifications(t *testing.T) {
	for ja, en := range map[string]string{
		"動画添削の依頼が届きました":     "New video-review request",
		"追加相談が届きました":        "New consultation received",
		"相談が届きました":          "New consultation request",
		"追加相談の期限が過ぎました":     "Consultation expired",
		"コーチが申し込みを承諾しました":   "Coach accepted your application",
		"コーチが依頼を返却しました":     "Coach returned the request",
		"無料相談にコーチの提案が届きました": "Coach sent a proposal",
		"1ヶ月プランが終了しました":     "1-month plan ended",
		"異議申立ての再審査が終わりました":  "Appeal review finished",
	} {
		got := localizePushMessage("en", push.Message{Title: ja, URL: "/coach"})
		if got.Title != en || got.Body != "" || got.URL != "/coach" {
			t.Fatalf("%s: %+v", ja, got)
		}
	}
	// 期間が終わる3日前の知らせだけは、ダウンロードの案内1行を本文に残す。
	got := localizePushMessage("en", push.Message{Title: "パーソナル指導の期間がもうすぐ終わります", Body: contractCloseNoticeBody})
	if got.Title != "Personal coaching ending soon" || got.Body != "Download your messages and menus before it ends" {
		t.Fatalf("close notice: %+v", got)
	}
}

// 投稿系のプッシュ（コメント・返信・練習・募集・ケガ）も表題1行だけで、本文は空（本体 #609）。
func TestPostPushMessageHasNoBody(t *testing.T) {
	id := uuid.New()
	got := postPushMessage("コメントがありました", id)
	if got.Title != "コメントがありました" || got.Body != "" || got.URL != "/posts/"+id.String() || got.Tag != "post-"+id.String() {
		t.Fatalf("post push: %+v", got)
	}
	en := localizePushMessage("en", got)
	if en.Body != "" {
		t.Fatalf("en body: %q", en.Body)
	}
}
