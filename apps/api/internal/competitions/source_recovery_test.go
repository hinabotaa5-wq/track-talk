package competitions

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIwateScheduleResolvesPDF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `<html><a href="/common/uploads/2026/schedule.pdf">2026年度 岩手陸上競技協会 主要競技会日程</a></html>`)
	}))
	defer server.Close()
	for _, source := range Sources {
		if source.Prefecture == "03" {
			source.PageURL = server.URL
			urls, err := source.ResolveURLs(context.Background())
			if err != nil || len(urls) != 1 || urls[0] != server.URL+"/common/uploads/2026/schedule.pdf" {
				t.Fatalf("%v %v", urls, err)
			}
			return
		}
	}
	t.Fatal("岩手の取り込み元なし")
}
