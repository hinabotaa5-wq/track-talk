package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestAdminDeleteUser(t *testing.T) {
	id := uuid.New()
	status := http.StatusOK
	var method, path, authz, apiKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		authz = r.Header.Get("Authorization")
		apiKey = r.Header.Get("apikey")
		http.Error(w, "nope", status)
	}))
	t.Cleanup(srv.Close)

	admin := Admin{BaseURL: srv.URL + "/", APIKey: "service-role", HTTP: srv.Client()}
	if err := admin.DeleteUser(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodDelete || path != "/auth/v1/admin/users/"+id.String() {
		t.Fatalf("request %s %s", method, path)
	}
	if authz != "Bearer service-role" || apiKey != "service-role" {
		t.Fatalf("headers %q %q", authz, apiKey)
	}

	status = http.StatusNotFound
	if err := admin.DeleteUser(context.Background(), id); err != nil {
		t.Fatalf("missing user: %v", err)
	}

	status = http.StatusInternalServerError
	if err := admin.DeleteUser(context.Background(), id); err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("upstream error: %v", err)
	}

	if err := (Admin{}).DeleteUser(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if !admin.Enabled() || (Admin{BaseURL: srv.URL}).Enabled() {
		t.Fatal("enabled")
	}
}
