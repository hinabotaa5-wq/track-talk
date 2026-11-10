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

// FindPDFLink は、pageURL からリンクされた PDF のうち、ファイル名かリンクの文字が pattern に合う、いちばん新しい年のものを返す。
// 協会は改訂のたびに PDF の名前を変えるので、変わらないのはリンクしているページの方。
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

// pdfLinks は、合う PDF のリンクを並べる。前の年の日程表が、今年のものの隣にリンクされたまま残っていることが多い。
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

// pageClient は県陸協などのページと PDF を取りに行く。応答しないサイトが1つあるだけで
// 取り込み全体の持ち時間を使い切らないように、本文を読み終えるまでの上限を付ける。
// 1ページの worst case が県あたりの上限（importcompetitions の SourceTimeout）を
// 食いつぶさないよう、Gemini の1回分（GeminiHTTPTimeout）と同じ30秒にする。
var pageClient = &http.Client{Timeout: 30 * time.Second}

// browserUserAgent は、協会サイトの bot 対策で 403 を返す場合の逃げ道。
// 通常は連絡先入りの userAgent で取りに行き、403 のときだけ1回だけ試す。
const browserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

func get(ctx context.Context, pageURL string) (*http.Response, string, error) {
	res, finalURL, err := doGet(ctx, pageURL, userAgent)
	if err != nil {
		return nil, "", err
	}
	if res.StatusCode != http.StatusForbidden {
		return checkGetResponse(res, finalURL, pageURL)
	}
	res.Body.Close()
	retry, retryURL, err := doGet(ctx, pageURL, browserUserAgent)
	if err != nil {
		return nil, "", err
	}
	return checkGetResponse(retry, retryURL, pageURL)
}

func doGet(ctx context.Context, pageURL, agent string) (*http.Response, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", agent)
	res, err := pageClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	finalURL := pageURL
	if res.Request != nil && res.Request.URL != nil {
		finalURL = res.Request.URL.String()
	}
	return res, finalURL, nil
}

func checkGetResponse(res *http.Response, finalURL, pageURL string) (*http.Response, string, error) {
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, "", fmt.Errorf("page %s: %s", pageURL, res.Status)
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

// PDFPage は、年間の日程表の PDF を包む。シーズンはファイル名から取り、名前に年がなければ今の4月〜3月のシーズンにする。
// ディレクトリの年はアップロードした日付（uploads/2026/10/）なので見ない。
func PDFPage(data []byte, pageURL string, now time.Time) (Page, error) {
	if !strings.HasPrefix(string(data), "%PDF") {
		return Page{}, fmt.Errorf("pdf %s: not a pdf", pageURL)
	}
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return Page{}, fmt.Errorf("page url: %w", err)
	}
	// 年度の境目は日本時間で決める（ランナーは UTC なので、日本時間の4月1日の朝は UTC ではまだ3月）。
	year := SeasonYear(now)
	if m := pdfNameYearPattern.FindStringSubmatch(path.Base(parsed.Path)); m != nil {
		if y, _ := strconv.Atoi(m[1]); y >= 2020 && y <= seasonYearCap() {
			year = y
		}
	}
	return Page{SeasonYear: year, PDFs: [][]byte{data}}, nil
}
