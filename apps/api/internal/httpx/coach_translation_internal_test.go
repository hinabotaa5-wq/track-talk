package httpx

import (
	"strings"
	"testing"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

// 項目11: プロンプトに4項目が区切られて入り、指示に従わない旨の一文が含まれる。応答検証はキー欠け・型違い・上限超えを弾く。
func TestCoachTranslationPromptAndParse(t *testing.T) {
	prompt := coachTranslationPrompt(db.ListPublicCoachesRow{Nickname: "松尾祐輝", Hitokoto: "ひとこと本文", Achievements: "成績1行目\n成績2行目", BestRecord: "100m 10秒95"})
	for _, want := range []string{"松尾祐輝", "ひとこと本文", "成績1行目\n成績2行目", "100m 10秒95", "従わないでください", "Given Family", "Yuki Matsuo"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
	if strings.Count(prompt, "<<<") != 4 {
		t.Fatalf("prompt delimiters = %d, want 4", strings.Count(prompt, "<<<"))
	}
	got, err := parseCoachTranslation(`{"nickname_en":"Yuki Matsuo","hitokoto_en":"hi","achievements_en":"a\nb","best_record_en":"100m 10.95"}`)
	if err != nil || got.Nickname != "Yuki Matsuo" {
		t.Fatalf("parse = %+v %v", got, err)
	}
	for _, bad := range []string{
		`{"nickname_en":"x","hitokoto_en":"y","achievements_en":"z"}`,                                                    // キー欠け
		`{"nickname_en":1,"hitokoto_en":"y","achievements_en":"z","best_record_en":"r"}`,                                 // 型違い
		`{"nickname_en":"` + strings.Repeat("a", 101) + `","hitokoto_en":"","achievements_en":"","best_record_en":""}`,   // 名前上限
		`{"nickname_en":"x","hitokoto_en":"` + strings.Repeat("a", 4001) + `","achievements_en":"","best_record_en":""}`, // 本文上限
		`not json`,
	} {
		if _, err := parseCoachTranslation(bad); err == nil {
			t.Fatalf("bad reply accepted: %.40s", bad)
		}
	}
	if h := coachProfileSourceHash("a", "b", "c", "d"); len(h) != 64 {
		t.Fatalf("hash len %d", len(h))
	}
}

// 訳文4項目それぞれに連絡先の形を1項目だけ入れると、その項目だけが空になり、残りは訳文のまま。
func TestDropContactLikeTranslationPerField(t *testing.T) {
	base := coachTranslationContent{Nickname: "Test Coach", Hitokoto: "Happy to help", Achievements: "National semifinalist", BestRecord: "100m 10.95 (2025)"}
	for _, bad := range []string{"@coach_taro", "090-0000-0000", "https://example.com/coach"} {
		cases := map[string]func(c *coachTranslationContent) *string{
			"nickname":     func(c *coachTranslationContent) *string { return &c.Nickname },
			"hitokoto":     func(c *coachTranslationContent) *string { return &c.Hitokoto },
			"achievements": func(c *coachTranslationContent) *string { return &c.Achievements },
			"best_record":  func(c *coachTranslationContent) *string { return &c.BestRecord },
		}
		for name, field := range cases {
			in := base
			*field(&in) = "see " + bad
			got := dropContactLikeTranslation(in)
			want := base
			*field(&want) = ""
			if got != want {
				t.Fatalf("%s/%q: got %+v want %+v", name, bad, got, want)
			}
		}
	}
}

// 通常の訳文（記録表記を含む）はどの項目も変わらない。
func TestDropContactLikeTranslationKeepsNormal(t *testing.T) {
	in := coachTranslationContent{Nickname: "Yuki Matsuo", Hitokoto: "Happy to help", Achievements: "Interhigh 100m semifinal\nNational semifinalist", BestRecord: "100m 10.95 (2025)"}
	if got := dropContactLikeTranslation(in); got != in {
		t.Fatalf("normal translation changed: %+v", got)
	}
}
