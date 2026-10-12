package competitions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

const wikiAPI = "https://ja.wikipedia.org/w/api.php"

// FetchMajorHosts reads one Wikipedia host table and keeps editions that have not ended.
// The photo is a city skyline from Wikimedia Commons. A missing photo is left blank.
func FetchMajorHosts(ctx context.Context, kind string, now time.Time) ([]MajorHost, error) {
	page, heading, parse := wikiSource(kind)
	if parse == nil {
		return nil, fmt.Errorf("開催地の種類が不明です")
	}
	body, err := fetchWikiSection(ctx, page, heading)
	if err != nil {
		return nil, err
	}
	hosts, err := parse(strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	upcoming := UpcomingMajorHosts(hosts, now)
	attachThumbnails(ctx, upcoming)
	return upcoming, nil
}

func wikiSource(kind string) (page, heading string, parse func(io.Reader) ([]MajorHost, error)) {
	switch kind {
	case MajorWorlds:
		return "世界陸上競技選手権大会", "大会一覧", ParseWorldChampionships
	case MajorOlympics:
		return "オリンピックの開催地選考", "夏季オリンピック", ParseSummerOlympics
	default:
		return "", "", nil
	}
}

func fetchWikiSection(ctx context.Context, page, heading string) (string, error) {
	toc, err := getBody(ctx, wikiParseURL(page, url.Values{"prop": {"tocdata"}}))
	if err != nil {
		return "", err
	}
	index, err := sectionIndex(toc, heading)
	if err != nil {
		return "", err
	}
	body, err := getBody(ctx, wikiParseURL(page, url.Values{
		"prop":    {"text"},
		"section": {index},
	}))
	if err != nil {
		return "", err
	}
	var parsed struct {
		Parse struct {
			Text map[string]string `json:"text"`
		} `json:"parse"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("節を読めません: %w", err)
	}
	htmlText := parsed.Parse.Text["*"]
	if strings.TrimSpace(htmlText) == "" {
		return "", fmt.Errorf("%s の節が空です", heading)
	}
	return htmlText, nil
}

func wikiParseURL(page string, extra url.Values) string {
	query := url.Values{}
	query.Set("action", "parse")
	query.Set("page", page)
	query.Set("format", "json")
	for key, values := range extra {
		for _, value := range values {
			query.Set(key, value)
		}
	}
	return wikiAPI + "?" + query.Encode()
}

func sectionIndex(body []byte, heading string) (string, error) {
	var parsed struct {
		Parse struct {
			TocData struct {
				Sections []struct {
					Line   string `json:"line"`
					Anchor string `json:"anchor"`
					Index  string `json:"index"`
				} `json:"sections"`
			} `json:"tocdata"`
		} `json:"parse"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("目次を読めません: %w", err)
	}
	for _, section := range parsed.Parse.TocData.Sections {
		if section.Anchor == heading || section.Line == heading {
			if section.Index == "" {
				break
			}
			return section.Index, nil
		}
	}
	return "", fmt.Errorf("%s の節が見つかりません", heading)
}

func attachThumbnails(ctx context.Context, hosts []MajorHost) {
	for i := range hosts {
		if image := cityLandscape(ctx, hosts[i]); image != "" {
			hosts[i].ImageURL = image
			continue
		}
		hosts[i].ImageURL = wikiCityLead(ctx, hosts[i])
	}
}

func photoPages(host MajorHost) []string {
	var pages []string
	if host.CityPageURL != "" {
		pages = append(pages, host.CityPageURL)
	}
	if host.PageURL != "" && host.PageURL != host.CityPageURL {
		pages = append(pages, host.PageURL)
	}
	return pages
}

func wikiThumbnail(ctx context.Context, pageURL string) (string, error) {
	title, ok := wikiTitle(pageURL)
	if !ok {
		return "", nil
	}
	return wikiSummaryThumbnail(ctx, title)
}

func wikiSummaryThumbnail(ctx context.Context, title string) (string, error) {
	body, err := getBody(ctx, "https://ja.wikipedia.org/api/rest_v1/page/summary/"+url.PathEscape(title))
	if err != nil {
		return "", err
	}
	return thumbnailSource(body), nil
}

func wikiTitle(pageURL string) (string, bool) {
	parsed, err := url.Parse(pageURL)
	if err != nil || !strings.HasPrefix(parsed.Path, "/wiki/") {
		return "", false
	}
	title, err := url.PathUnescape(strings.TrimPrefix(parsed.Path, "/wiki/"))
	if err != nil || title == "" {
		return "", false
	}
	return title, true
}

func thumbnailSource(body []byte) string {
	var parsed struct {
		Type      string `json:"type"`
		Thumbnail struct {
			Source string `json:"source"`
		} `json:"thumbnail"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.Type == "disambiguation" {
		return ""
	}
	return strings.TrimSpace(parsed.Thumbnail.Source)
}
