package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunRejectsBadInvocation(t *testing.T) {
	env := map[string]string{
		"IDENTITY_MAIN_READ_DATABASE_URL": "postgres://reader:mainsecret@db.example/postgres?sslmode=require",
		"IDENTITY_DATABASE_URL":           "postgres://app:vaultsecret@vault.example/postgres?sslmode=require",
		"IDENTITY_AUTH_URL":               "https://vault.example",
		"IDENTITY_AUTH_ADMIN_KEY":         "adminsecret",
		"IDENTITY_AUTH_PUBLIC_KEY":        "publickey",
	}
	get := func(k string) string { return env[k] }
	for _, c := range []struct {
		args []string
		want string
	}{
		{nil, "usage"},
		{[]string{"provision", "not-a-uuid", "aaa"}, "coach_id_invalid"},
		{[]string{"provision", "00000000-0000-0000-0000-000000000000", "aaa"}, "coach_id_invalid"},
		{[]string{"promote", "2877c3f1-0000-4000-8000-000000000000", "aaa"}, "unknown_op"},
	} {
		var buf bytes.Buffer
		if code := run(c.args, get, &buf); code != 2 {
			t.Errorf("%v: exit %d", c.args, code)
		}
		var out map[string]any
		if err := json.Unmarshal(buf.Bytes(), &out); err != nil || out["error"] != c.want || out["event"] != "identity_synthetic" {
			t.Errorf("%v: %s", c.args, buf.String())
		}
	}
	// 同じ接続先・管理用キーと公開キーの取り違えは起動前に止め、値を出さない。
	env["IDENTITY_AUTH_ADMIN_KEY"] = "publickey"
	var buf bytes.Buffer
	if code := run([]string{"verify", "2877c3f1-0000-4000-8000-000000000000", "aaa"}, get, &buf); code != 2 {
		t.Fatalf("exit %d", code)
	}
	for _, secret := range []string{"mainsecret", "vaultsecret", "publickey", "adminsecret"} {
		if strings.Contains(buf.String(), secret) {
			t.Fatalf("output leaks %s", secret)
		}
	}
	env["IDENTITY_AUTH_ADMIN_KEY"] = "adminsecret"
	env["IDENTITY_DATABASE_URL"] = env["IDENTITY_MAIN_READ_DATABASE_URL"]
	buf.Reset()
	if code := run([]string{"verify", "2877c3f1-0000-4000-8000-000000000000", "aaa"}, get, &buf); code != 2 || !strings.Contains(buf.String(), "database_configuration_invalid") {
		t.Fatalf("same database accepted: %s", buf.String())
	}
}
