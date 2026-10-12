package competitions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPDFPageSeason(t *testing.T) {
	pdf := []byte("%PDF-1.7 schedule")
	tests := []struct {
		url  string
		now  time.Time
		want int
	}{
		{"https://example.com/pdf/2026/2026_schedule_hp2.pdf", time.Date(2027, 5, 1, 0, 0, 0, 0, time.UTC), 2026},
		{"https://example.com/schedule_hokkaido2026df0814.pdf", time.Date(2027, 5, 1, 0, 0, 0, 0, time.UTC), 2026},
		// The upload directory is the posting month, not the season.
		{"https://example.com/wp-content/uploads/2027/02/6d5f1d6ce9518a2b.pdf", time.Date(2027, 2, 10, 0, 0, 0, 0, time.UTC), 2026},
		{"https://example.com/competition_16_20260824.pdf", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), 2026},
	}
	for _, tt := range tests {
		page, err := PDFPage(pdf, tt.url, tt.now)
		if err != nil {
			t.Fatal(err)
		}
		if page.SeasonYear != tt.want || len(page.PDFs) != 1 {
			t.Fatalf("%s: season %d pdfs %d, want %d", tt.url, page.SeasonYear, len(page.PDFs), tt.want)
		}
	}
	if _, err := PDFPage([]byte("<html>"), "https://example.com/a.pdf", time.Now()); err == nil {
		t.Fatal("html accepted as pdf")
	}
}

func TestFindPDFLinkPicksNewestMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<ul>
<li><a href="pdf/2016schedule.pdf">日程</a></li>
<li><a href="pdf/2025/2025_schedule_hp2.pdf">2025年度日程</a></li>
<li><a href="pdf/2026/2026_schedule_hp2.pdf">2026年度日程</a></li>
<li><a href="uploads/x1.pdf">２０２６年度　競技会コード</a></li>
<li><a href="uploads/x2.pdf">２０２６年度　競技会日程</a></li>
</ul>`))
	}))
	defer server.Close()

	link, err := FindPDFLink(context.Background(), server.URL+"/", `_schedule_hp`)
	if err != nil {
		t.Fatal(err)
	}
	if link != server.URL+"/pdf/2026/2026_schedule_hp2.pdf" {
		t.Fatalf("link %s", link)
	}
	link, err = FindPDFLink(context.Background(), server.URL+"/", `競技会日程`)
	if err != nil || link != server.URL+"/uploads/x2.pdf" {
		t.Fatalf("link %s err %v", link, err)
	}
	if _, err := FindPDFLink(context.Background(), server.URL+"/", `行事予定`); err == nil {
		t.Fatal("missing link found")
	}
}

func TestExtractSendsPDFsInline(t *testing.T) {
	var got []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Contents []struct {
				Parts []struct {
					Text       string `json:"text"`
					InlineData struct {
						MimeType string `json:"mime_type"`
						Data     string `json:"data"`
					} `json:"inline_data"`
				} `json:"parts"`
			} `json:"contents"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		for _, part := range body.Contents[0].Parts {
			if part.InlineData.MimeType == "application/pdf" {
				data, _ := base64.StdEncoding.DecodeString(part.InlineData.Data)
				got = append(got, string(data))
			} else if !strings.Contains(part.Text, "添付のPDF") {
				t.Fatalf("prompt has no pdf note: %s", part.Text)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"meets\":[]}"}]}}]}`))
	}))
	defer server.Close()
	original := generateEndpoint
	generateEndpoint = server.URL
	defer func() { generateEndpoint = original }()

	client := Client{Key: "test", HTTP: server.Client()}
	page := Page{SeasonYear: 2026, PDFs: [][]byte{[]byte("%PDF a"), []byte("%PDF b")}}
	if _, err := client.Extract(context.Background(), page); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "%PDF a" || got[1] != "%PDF b" {
		t.Fatalf("pdfs %q", got)
	}
}
