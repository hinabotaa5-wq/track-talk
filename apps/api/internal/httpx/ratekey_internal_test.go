package httpx

import "testing"

func TestRateKeyGroupsIPv6ByPrefix(t *testing.T) {
	a := rateKey("2001:db8:1:2:aaaa:bbbb:cccc:dddd")
	b := rateKey("2001:db8:1:2:1111:2222:3333:4444")
	if a != b {
		t.Fatalf("same /64 should share a key: %q %q", a, b)
	}
	if c := rateKey("2001:db8:1:3::1"); c == a {
		t.Fatalf("another /64 should not share a key: %q", c)
	}
	if got := rateKey("203.0.113.9"); got != "203.0.113.9" {
		t.Fatalf("ipv4 %q", got)
	}
}
