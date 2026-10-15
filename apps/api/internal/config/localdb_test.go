package config

import "testing"

func TestRequireLocalDatabase(t *testing.T) {
	t.Setenv("ALLOW_REMOTE_DB", "")
	for dsn, ok := range map[string]bool{
		"postgres://u:p@localhost:5432/db":                                      true,
		"postgres://u:p@127.0.0.1:55432/db?sslmode=disable":                     true,
		"postgres://u:p@[::1]:5432/db":                                          true,
		"postgres://u:p@aws-0-ap-southeast-1.pooler.supabase.com:5432/postgres": false,
		"postgres://u:p@localhost/db?host=evil.example.com":                     false,
		"postgres://u:p@localhost/db?hostaddr=203.0.113.5":                      false,
		"postgres://u:p@localhost,evil.example.com/db":                          false,
		"host=evil.example.com user=u dbname=x":                                 false,
		"postgres:///db":                                                        false,
		"postgres://u:p@db.example.com/db":                                      false,
	} {
		if got := RequireLocalDatabase(dsn) == nil; got != ok {
			t.Errorf("%s: got %v want %v", dsn, got, ok)
		}
	}
	t.Setenv("ALLOW_REMOTE_DB", "1")
	if err := RequireLocalDatabase("postgres://u:p@db.example.com/db"); err != nil {
		t.Errorf("ALLOW_REMOTE_DB=1 でも拒否された: %v", err)
	}
}
