package cloudinary

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStoredAssetPagesAndIndividualDeletionResults(t *testing.T) {
	id := NewPublicID()
	status := "failed"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, secret, _ := r.BasicAuth()
		if key != "key" || secret != "secret" {
			t.Error("missing authentication")
		}
		if r.Method == http.MethodGet {
			if r.URL.Query().Get("prefix") != Folder+"/" || r.URL.Query().Get("next_cursor") != "page2" {
				t.Error(r.URL)
			}
			json.NewEncoder(w).Encode(map[string]any{"resources": []StoredAsset{{PublicID: id, CreatedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}}, "next_cursor": "page3"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"deleted": map[string]string{id: status}})
	}))
	defer server.Close()
	c := Client{Cloud: "demo", Key: "key", Secret: "secret", APIBase: server.URL}
	rows, next, err := c.ListStoredAssets(context.Background(), "image", "upload", "page2")
	if err != nil || next != "page3" || len(rows) != 1 || rows[0].CreatedAt.IsZero() {
		t.Fatalf("%+v %s %v", rows, next, err)
	}
	if err := c.DeleteStoredAsset(context.Background(), "image", "upload", id); err == nil {
		t.Fatal("unconfirmed deletion accepted")
	}
	for _, result := range []string{"deleted", "not_found"} {
		status = result
		if err := c.DeleteStoredAsset(context.Background(), "image", "upload", id); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.DeleteStoredAsset(context.Background(), "image", "upload", "foreign/file"); err == nil {
		t.Fatal("foreign asset accepted")
	}
}
