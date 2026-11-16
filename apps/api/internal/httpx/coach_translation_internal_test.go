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
		`{"nickname_en":"x","hitokoto_en":"y","achievements_en":"z"}`,                                  // キー欠け
		`{"nickname_en":1,"hitokoto_en":"y","achievements_en":"z","best_record_en":"r"}`,              // 型違い
		`{"nickname_en":"` + strings.Repeat("a", 101) + `","hitokoto_en":"","achievements_en":"","best_record_en":""}`, // 名前上限
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
