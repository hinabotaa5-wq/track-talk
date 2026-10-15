package httpx

import (
	"strings"
	"testing"
)

func TestTitleFromBody(t *testing.T) {
	if got := titleFromBody("  今日は\n調子が良い  "); got != "今日は 調子が良い" {
		t.Errorf("got %q", got)
	}
	if got := titleFromBody(""); got != "" {
		t.Errorf("got %q", got)
	}
	got := titleFromBody(strings.Repeat("あ", 50))
	if got != strings.Repeat("あ", 40)+"…" {
		t.Errorf("got %q", got)
	}
}
