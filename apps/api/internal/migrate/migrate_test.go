package migrate

import "testing"

func TestWithAppName(t *testing.T) {
	got := withAppName("postgres://u:p@localhost:5432/db?sslmode=disable")
	if got != "postgres://u:p@localhost:5432/db?application_name=track-talk-migrate&sslmode=disable" {
		t.Fatalf("got %q", got)
	}
	// 既に名札があるものは変えない。
	in := "postgres://u:p@localhost:5432/db?application_name=custom"
	if got := withAppName(in); got != in {
		t.Fatalf("got %q, want unchanged", got)
	}
	if got := withAppName("not a url"); got != "not a url" {
		t.Fatalf("got %q, want unchanged", got)
	}
}
