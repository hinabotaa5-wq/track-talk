package competitions

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type LinkRef struct {
	URL    string
	Text   string
	Header string
}

type cell struct {
	Header string
	Text   string
	Links  []LinkRef
}

type row struct {
	Cells []cell
}

// Page is the schedule text and links taken from one association homepage.
type Page struct {
	SeasonYear int
	Body       string
	Links      []LinkRef
	rows       []row
}

var seasonYearPattern = regexp.MustCompile(`(20\d{2})年`)

func ParsePage(r io.Reader, pageURL string) (Page, error) {
	base, err := url.Parse(pageURL)
	if err != nil {
		return Page{}, fmt.Errorf("page url: %w", err)
	}
	doc, err := html.Parse(r)
	if err != nil {
		return Page{}, fmt.Errorf("parse html: %w", err)
	}
	year, ok := findSeasonYear(doc)
	if !ok {
		return Page{}, fmt.Errorf("競技年度が見つかりません")
	}
	var rows []row
	var links []LinkRef
	for _, table := range scheduleTables(doc) {
		headers, headerAt := tableHeaders(table)
		if !isScheduleHeader(headers) {
			continue
		}
		for i, tr := range directRows(table) {
			if i <= headerAt {
				continue
			}
			cells := elementChildren(tr, "td")
			if len(cells) == 0 {
				continue
			}
			var parsed row
			for j, node := range cells {
				header := ""
				if j < len(headers) {
					header = headers[j]
				}
				text := cellText(node)
				cellLinks := cellLinks(node, base, header)
				links = append(links, cellLinks...)
				parsed.Cells = append(parsed.Cells, cell{
					Header: header,
					Text:   text,
					Links:  cellLinks,
				})
			}
			if len(parsed.Cells) > 0 {
				rows = append(rows, parsed)
			}
		}
	}
	if len(rows) == 0 {
		return Page{}, fmt.Errorf("競技日程の表が見つかりません")
	}
	return Page{SeasonYear: year, Body: renderRows(rows), Links: links, rows: rows}, nil
}

func renderRows(rows []row) string {
	var b strings.Builder
	for _, row := range rows {
		for _, cell := range row.Cells {
			fmt.Fprintf(&b, "%s: %s", cell.Header, cell.Text)
			for _, link := range cell.Links {
				if link.Text != "" && link.Text != cell.Text {
					fmt.Fprintf(&b, " %s", link.Text)
				}
				fmt.Fprintf(&b, " %s", link.URL)
			}
			b.WriteByte('\n')
		}
		b.WriteString("---\n")
	}
	return b.String()
}

func findSeasonYear(doc *html.Node) (int, bool) {
	text := normalizeDigits(nodeText(doc))
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(20\d{2})年競技`),
		regexp.MustCompile(`(20\d{2})年度`),
		seasonYearPattern,
	}
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(text)
		if match == nil {
			continue
		}
		year, err := strconv.Atoi(match[1])
		if err == nil && year >= 1990 && year <= 2100 {
			return year, true
		}
	}
	return 0, false
}

func scheduleTables(doc *html.Node) []*html.Node {
	var tables []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			headers, _ := tableHeaders(n)
			if isScheduleHeader(headers) {
				tables = append(tables, n)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return tables
}

func isScheduleHeader(headers []string) bool {
	for _, header := range headers {
		header = strings.TrimSpace(header)
		if header == "日" || strings.Contains(header, "開催日") || strings.Contains(header, "要項") {
			return true
		}
	}
	return false
}

func tableHeaders(table *html.Node) ([]string, int) {
	rows := directRows(table)
	for i, row := range rows {
		cells := elementChildren(row, "th")
		if len(cells) == 0 {
			continue
		}
		headers := make([]string, len(cells))
		for j, cell := range cells {
			headers[j] = cellText(cell)
		}
		return headers, i
	}
	if len(rows) == 0 {
		return nil, 0
	}
	cells := elementChildren(rows[0], "td")
	headers := make([]string, len(cells))
	for i, cell := range cells {
		headers[i] = cellText(cell)
	}
	return headers, 0
}

func directRows(table *html.Node) []*html.Node {
	var rows []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n != table && n.Type == html.ElementNode && n.Data == "table" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "tr" {
			rows = append(rows, n)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(table)
	return rows
}

func elementChildren(n *html.Node, tag string) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			out = append(out, c)
		}
	}
	if len(out) == 0 && (tag == "td" || tag == "th") {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
				out = append(out, c)
			}
		}
	}
	return out
}

func cellText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && isStruck(n) {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		if n.Type == html.ElementNode && n.Data == "br" {
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func cellLinks(n *html.Node, base *url.URL, header string) []LinkRef {
	var links []LinkRef
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && isStruck(n) {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key != "href" {
					continue
				}
				abs, ok := absoluteURL(base, attr.Val)
				if !ok {
					continue
				}
				links = append(links, LinkRef{URL: abs, Text: cellText(n), Header: header})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return links
}

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

func isStruck(n *html.Node) bool {
	return n.Data == "del" || n.Data == "s" || n.Data == "strike"
}

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

func normalizeDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '０' && r <= '９' {
			b.WriteRune('0' + (r - '０'))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
