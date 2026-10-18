package competitions

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

const maxPDFBytes = 15 << 20

func FetchPage(ctx context.Context, pageURL string) (Page, error) {
	res, finalURL, err := get(ctx, pageURL)
	if err != nil {
		return Page{}, err
	}
	defer res.Body.Close()
	if isPDFResponse(res, finalURL) {
		data, err := io.ReadAll(io.LimitReader(res.Body, maxPDFBytes+1))
		if err != nil {
			return Page{}, err
		}
		if len(data) > maxPDFBytes {
			return Page{}, fmt.Errorf("pdf %s: too large", pageURL)
		}
		return PDFPage(data, finalURL, time.Now())
	}
	reader, err := charset.NewReader(res.Body, res.Header.Get("Content-Type"))
	if err != nil {
		return Page{}, err
	}
	return ParsePage(io.LimitReader(reader, 8<<20), finalURL)
}

// FindPDFLink returns the newest-year PDF linked from pageURL whose file name or link text matches pattern.
// Associations rename the PDF on every revision, so the page that links to it is what stays put.
func FindPDFLink(ctx context.Context, pageURL, pattern string) (string, error) {
	match, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	res, finalURL, err := get(ctx, pageURL)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	reader, err := charset.NewReader(res.Body, res.Header.Get("Content-Type"))
	if err != nil {
		return "", err
	}
	doc, err := html.Parse(io.LimitReader(reader, 8<<20))
	if err != nil {
		return "", err
	}
	base, err := url.Parse(finalURL)
	if err != nil {
		return "", err
	}
	link, newest := "", 0
	for _, candidate := range pdfLinks(doc, base, match) {
		year := 0
		for _, m := range anyYearPattern.FindAllStringSubmatch(candidate.label, -1) {
			if y, _ := strconv.Atoi(m[1]); y > year {
				year = y
			}
		}
		if link == "" || year > newest {
			link, newest = candidate.url, year
		}
	}
	if link == "" {
		return "", fmt.Errorf("%s に %s のPDFリンクがありません", pageURL, pattern)
	}
	return link, nil
}

var (
	anyYearPattern     = regexp.MustCompile(`(20\d{2})`)
	pdfNameYearPattern = regexp.MustCompile(`(?:^|[^0-9a-fA-F])(20\d{2})(?:[^0-9]|$)`)
)

type pdfCandidate struct {
	url   string
	label string
}

// pdfLinks lists matching PDF links. Old years' schedules often stay linked next to the current one.
func pdfLinks(n *html.Node, base *url.URL, match *regexp.Regexp) []pdfCandidate {
	var out []pdfCandidate
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, attr := range n.Attr {
			if attr.Key != "href" {
				continue
			}
			ref, err := base.Parse(strings.TrimSpace(attr.Val))
			if err != nil || !strings.HasSuffix(strings.ToLower(ref.Path), ".pdf") {
				break
			}
			label := normalizeDigits(path.Base(ref.Path) + " " + cellText(n))
			if match.MatchString(label) {
				out = append(out, pdfCandidate{url: ref.String(), label: label})
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out = append(out, pdfLinks(c, base, match)...)
	}
	return out
}

func get(ctx context.Context, pageURL string) (*http.Response, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", userAgent)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, "", fmt.Errorf("page %s: %s", pageURL, res.Status)
	}
	finalURL := pageURL
	if res.Request != nil && res.Request.URL != nil {
		finalURL = res.Request.URL.String()
	}
	return res, finalURL, nil
}

func isPDFResponse(res *http.Response, pageURL string) bool {
	if strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "application/pdf") {
		return true
	}
	parsed, err := url.Parse(pageURL)
	return err == nil && strings.HasSuffix(strings.ToLower(parsed.Path), ".pdf")
}

// PDFPage wraps a yearly schedule PDF. The season comes from the file name, or the current April-to-March
// season when the name has no year. Directory years are ignored because they are upload dates (uploads/2026/10/).
func PDFPage(data []byte, pageURL string, now time.Time) (Page, error) {
	if !strings.HasPrefix(string(data), "%PDF") {
		return Page{}, fmt.Errorf("pdf %s: not a pdf", pageURL)
	}
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return Page{}, fmt.Errorf("page url: %w", err)
	}
	year := now.Year()
	if now.Month() < time.April {
		year--
	}
	if m := pdfNameYearPattern.FindStringSubmatch(path.Base(parsed.Path)); m != nil {
		if y, _ := strconv.Atoi(m[1]); y >= 2020 && y <= seasonYearCap() {
			year = y
		}
	}
	return Page{SeasonYear: year, PDFs: [][]byte{data}}, nil
}
