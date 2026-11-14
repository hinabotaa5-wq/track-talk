package httpx

import (
	"strings"
	"testing"
)

// 本体 #713：SNS・ホームページ欄の保存時 scheme ゲート。
// URL 形式の入力は http/https だけ許可し、それ以外の scheme・
// http/https でもホストが無いもの・userinfo を含むものは項目エラーにする。
// URL 形式でないテキスト（アカウント名・空白を含む文章・ドメインとポートらしい文字列）は許可する。
func TestNormalizeCoachSnsEntriesSchemeGate(t *testing.T) {
	const schemeErr = "SNS・ホームページの URL は http:// または https:// で始まるものだけ入力できます"

	reject := []struct {
		name  string
		entry string
	}{
		{"javascript", "javascript:alert(1)"},
		{"javascriptの大文字混じり", "JaVaScRiPt:alert(1)"},
		{"data", "data:text/plain;base64,AAAA"},
		{"vbscript", "vbscript:msgbox(1)"},
		{"file", "file:///etc/passwd"},
		{"ftp", "ftp://example.com/x"},
		{"mailto", "mailto:someone@example.com"},
		{"tel", "tel:+819012345678"},
		{"httpsでもuserinfoあり", "https://user:pass@example.com/"},
		{"httpsでもホストなし", "https:///path"},
		{"httpの大文字でもホストなし", "HTTP:///path"},
	}
	for _, tc := range reject {
		if got, err := normalizeCoachSnsEntries([]string{tc.entry}); err != schemeErr {
			t.Errorf("%s: got entries=%v err=%q, want err=%q", tc.name, got, err, schemeErr)
		}
	}

	// 有効な URL と不正な scheme の混在も、項目エラーで保存できない。
	if got, err := normalizeCoachSnsEntries([]string{"https://example.com/", "javascript:alert(1)"}); err != schemeErr {
		t.Errorf("mixed: got entries=%v err=%q, want err=%q", got, err, schemeErr)
	}

	allow := []struct {
		name  string
		entry string
	}{
		{"httpsの通常URL", "https://www.instagram.com/track.talk/"},
		{"httpの通常URL", "http://example.com/page?x=1"},
		{"YouTubeチャンネルURL", "https://youtube.com/channel/UCxxxxxxxxxxxxxxxxxxxxxx"},
		{"アカウント名", "@tracktalk.office"},
		{"schemeなしのドメイン", "youtube.com/@tracktalk"},
		{"空白を含む文章", "YouTube: チャンネルの紹介です"},
		{"ドメインとポートらしいテキスト", "example.com:8080"},
		{"大文字のhttps", "HTTPS://Example.COM/Path"},
		{"ポート付きhttp", "http://example.com:8080/x"},
	}
	for _, tc := range allow {
		got, err := normalizeCoachSnsEntries([]string{tc.entry})
		if err != "" {
			t.Errorf("%s: unexpected err=%q", tc.name, err)
			continue
		}
		if len(got) != 1 || got[0] != tc.entry {
			t.Errorf("%s: got %v, want [%s]", tc.name, got, tc.entry)
		}
	}
}

// 既存ルール（0件・4件・301文字・重複・空行）は新ゲートと両立すること。
func TestNormalizeCoachSnsEntriesKeepsExistingRules(t *testing.T) {
	if _, err := normalizeCoachSnsEntries(nil); err != "SNS・ホームページを1件以上入力してください" {
		t.Errorf("empty: err=%q", err)
	}
	if _, err := normalizeCoachSnsEntries([]string{"a", "b", "c", "d"}); err != "SNS・ホームページは3件まで入力できます" {
		t.Errorf("four: err=%q", err)
	}
	if _, err := normalizeCoachSnsEntries([]string{strings.Repeat("あ", 301)}); err != "SNS・ホームページの各項目は300文字以内で入力してください" {
		t.Errorf("long: err=%q", err)
	}
	got, err := normalizeCoachSnsEntries([]string{" @tracktalk.office ", "", "https://www.instagram.com/testcoach/", "所属クラブの公式アカウント", "@tracktalk.office"})
	if err != "" {
		t.Fatalf("dedup: err=%q", err)
	}
	want := "@tracktalk.office,https://www.instagram.com/testcoach/,所属クラブの公式アカウント"
	if strings.Join(got, ",") != want {
		t.Errorf("dedup: got %v", got)
	}
}
