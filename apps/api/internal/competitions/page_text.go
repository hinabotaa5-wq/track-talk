package competitions

import (
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/jst"
	"golang.org/x/net/html"
)

// maxTextBody は、とても長いページを Gemini のリクエスト1回で読める長さに収める。
const maxTextBody = 400 << 10

// plainTextPage は、日程の表がないページを文字の行にする。
// 表のセルは " | " でつなぎ、リンクのあとにはその URL を付けて、Gemini が大会を日付・会場・要項と組み合わせられるようにする。
func plainTextPage(doc *html.Node, base *url.URL) (string, []LinkRef) {
	var lines []string
	var links []LinkRef
	var line strings.Builder
	flush := func() {
		text := strings.Join(strings.Fields(line.String()), " ")
		line.Reset()
		if text == "" {
			return
		}
		if n := len(lines); n > 0 && lines[n-1] == text {
			return
		}
		lines = append(lines, text)
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			line.WriteString(n.Data)
			return
		}
		if n.Type != html.ElementNode && n.Type != html.DocumentNode {
			return
		}
		if n.Type == html.ElementNode {
			if skipTextTag(n.Data) || isStruck(n) || isCalendarWidget(n) {
				return
			}
			if n.Data == "br" || isBlockTag(n.Data) {
				flush()
			}
			if n.Data == "td" || n.Data == "th" {
				line.WriteString(" | ")
			}
			if n.Data == "a" {
				text := cellText(n)
				for _, attr := range n.Attr {
					if attr.Key != "href" {
						continue
					}
					abs, ok := absoluteURL(base, attr.Val)
					if !ok {
						continue
					}
					links = append(links, LinkRef{URL: abs, Text: text})
					line.WriteString(" " + text + " " + abs + " ")
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && isBlockTag(n.Data) {
			flush()
		}
	}
	walk(doc)
	flush()
	var body strings.Builder
	for _, text := range lines {
		if body.Len()+len(text) > maxTextBody {
			break
		}
		body.WriteString(text)
		body.WriteByte('\n')
	}
	return body.String(), links
}

func skipTextTag(tag string) bool {
	switch tag {
	case "head", "script", "style", "noscript", "nav", "footer", "svg", "select", "template", "iframe":
		return true
	}
	return false
}

func isBlockTag(tag string) bool {
	switch tag {
	case "div", "p", "li", "ul", "ol", "h1", "h2", "h3", "h4", "h5", "h6", "article", "section",
		"main", "tr", "table", "dl", "dt", "dd", "blockquote", "header", "aside", "figure":
		return true
	}
	return false
}

var scheduleDatePattern = regexp.MustCompile(`\d{1,2}\s*月\s*\d{1,2}\s*日|\d{1,2}\s*/\s*\d{1,2}|\d{4}\s*[./-]\s*\d{1,2}\s*[./-]\s*\d{1,2}`)

// looksLikeSchedule は、文字に大会の一覧と言えるだけの日付があるかを返す。
// 日付が少ししかないページ（ニュースのページ、エラーのページ）は Gemini に送らない。
func looksLikeSchedule(body string) bool {
	return len(scheduleDatePattern.FindAllStringIndex(normalizeDigits(body), -1)) >= 10
}

var seasonNamePattern = regexp.MustCompile(`(?:(?:R|令和)\s*\d{1,2}\s*[（(]\s*(20\d{2})\s*[）)]|(20\d{2})\s*(?:[（(][^）)]{0,8}[）)])?)\s*(?:年度|年開催)`)

// newestSeasonYear は、2026年度、2026年開催、2026（令和8）年度、R8（2026）年度の形で書かれた年のうち、いちばん大きいもの。
// 次のシーズンを上限にして、先の年へのナビゲーションのリンク（や 2029 のような仮の値）は無視する。
func newestSeasonYear(doc *html.Node) (int, bool) {
	newest := 0
	maxYear := seasonYearCap()
	for _, match := range seasonNamePattern.FindAllStringSubmatch(normalizeDigits(nodeText(doc)), -1) {
		yearStr := match[1]
		if yearStr == "" {
			yearStr = match[2]
		}
		year := 0
		for _, r := range yearStr {
			year = year*10 + int(r-'0')
		}
		if year > newest && year <= maxYear {
			newest = year
		}
	}
	return newest, newest != 0
}

func seasonYearCap() int {
	return time.Now().In(jst.Zone).Year() + 1
}
