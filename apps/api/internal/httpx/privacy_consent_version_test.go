package httpx

import (
	"os"
	"strings"
	"testing"
)

// 登録画面は同意した版をアカウントに残し、API はいまの版と同じときだけ記録する。
// 版を上げるときは Web 側(apps/web/lib/overseas-consent.ts)もそろえる。
func TestOverseasConsentVersionMatchesWeb(t *testing.T) {
	src, err := os.ReadFile("../../../web/lib/overseas-consent.ts")
	if err != nil {
		t.Fatalf("read web: %v", err)
	}
	want := `export const overseasConsentVersion = "` + overseasConsentVersion + `";`
	if !strings.Contains(string(src), want) {
		t.Fatalf("Web の overseasConsentVersion が API の %q とそろっていない", overseasConsentVersion)
	}
}
