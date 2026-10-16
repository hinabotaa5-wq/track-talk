package httpx

import (
	"slices"
	"testing"
)

func TestUnusedKeepsIDsStillInUse(t *testing.T) {
	got := unused([]string{"a", "b", "c"}, []string{"b"})
	if !slices.Equal(got, []string{"a", "c"}) {
		t.Fatalf("got %v", got)
	}
	if got := unused([]string{"a"}, []string{"a"}); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}
