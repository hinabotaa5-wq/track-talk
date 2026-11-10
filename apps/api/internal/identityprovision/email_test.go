package identityprovision

import (
	"testing"
	"time"
)

func TestSyntheticEmail(t *testing.T) {
	got, err := SyntheticEmail(" Ops.Inbox@Example.test ", "aaa")
	if err != nil || got != "ops.inbox+synthetic-aaa@example.test" {
		t.Fatalf("%q %v", got, err)
	}
	for _, tag := range []string{"", "AAA", "a-b", "a@b", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		if _, err := SyntheticEmail("ops.inbox@example.test", tag); err == nil {
			t.Errorf("tag %q accepted", tag)
		}
	}
	for _, base := range []string{"", "ops.inbox+x@example.test", "no-at", "a@localhost"} {
		if _, err := SyntheticEmail(base, "aaa"); err == nil {
			t.Errorf("base %q accepted", base)
		}
	}
	for email, want := range map[string]bool{
		"ops.inbox+synthetic-aaa@example.test":   true,
		"OPS.INBOX+SYNTHETIC-AAA@EXAMPLE.TEST":   true,
		"ops.inbox@example.test":                 false,
		"ops.inbox+synthetic@example.test":       false,
		"ops.inbox+synthetic-aaa@other.test":     false,
		"other+synthetic-aaa@example.test":       false,
		"ops.inbox+synthetic-aaa@example.test.x": false,
		"xops.inbox+synthetic-aaa@example.test":  false,
	} {
		if got := IsSynthetic("ops.inbox@example.test", email); got != want {
			t.Errorf("IsSynthetic(%q) = %v", email, got)
		}
	}
	if IsSynthetic("", "ops.inbox+synthetic-aaa@example.test") {
		t.Error("empty base accepted")
	}
}

// RFC 6238 付録 B の SHA-1 の値（8桁の下6桁）。
func TestTOTPCode(t *testing.T) {
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	for unix, want := range map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"} {
		got, err := totpCode(secret, time.Unix(unix, 0))
		if err != nil || got != want {
			t.Errorf("T=%d: %q %v, want %q", unix, got, err, want)
		}
	}
}
