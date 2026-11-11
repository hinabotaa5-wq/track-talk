package competitions

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// absoluteURL は、ページ内のリンクを絶対 URL に直す。
// 県別取り込みの削除後も、国際大会の取り込みで使う。
func absoluteURL(base *url.URL, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(strings.ToLower(raw), "javascript:") {
		return "", false
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	abs := base.ResolveReference(ref)
	if abs.Scheme != "http" && abs.Scheme != "https" {
		return "", false
	}
	abs.Fragment = ""
	return abs.String(), true
}

// nodeText は、HTML の要素の中のテキストを空白区切りでつなげて返す。
// 県別取り込みの削除後も、国内主要大会の取り込みで使う。
func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}
