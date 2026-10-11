package competitions

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/net/html/charset"
)

func FetchPage(ctx context.Context, pageURL string) (Page, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return Page{}, err
	}
	req.Header.Set("User-Agent", "TrackTalk/1.0")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Page{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Page{}, fmt.Errorf("page %s: %s", pageURL, res.Status)
	}
	reader, err := charset.NewReader(res.Body, res.Header.Get("Content-Type"))
	if err != nil {
		return Page{}, err
	}
	finalURL := pageURL
	if res.Request != nil && res.Request.URL != nil {
		finalURL = res.Request.URL.String()
	}
	return ParsePage(io.LimitReader(reader, 8<<20), finalURL)
}
