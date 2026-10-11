package competitions

import (
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const KagoshimaSourceURL = "https://kariku46.com/competition/#competition-schedule-results"
const KagoshimaPageURL = "https://kariku46.com/competition/"

type Meet struct {
	Name         string
	StartsOn     time.Time
	EndsOn       *time.Time
	Location     string
	Prefecture   string
	SeasonYear   int
	OrganizerURL string
	SourceURL    string
	LinkLabel    string
}

var (
	seasonYearPattern = regexp.MustCompile(`(\d{4})年競技日程`)
	datePattern       = regexp.MustCompile(`^(\d{1,2})/(\d{1,2})(?:[-〜～~](\d{1,2}))?$`)
)

func ParseKagoshima(r io.Reader, sourceURL string) ([]Meet, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	season, err := findSeasonYear(doc)
	if err != nil {
		return nil, err
	}
	table := findScheduleTable(doc)
	if table == nil {
		return nil, fmt.Errorf("競技日程の表が見つかりません")
	}
	var meets []Meet
	for _, row := range tableRows(table) {
		cells := rowCells(row)
		if len(cells) < 3 {
			continue
		}
		start, end, err := parseDate(cellText(cells[0]), season)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", cellText(cells[0]), err)
		}
		name := cellText(cells[1])
		if name == "" {
			return nil, fmt.Errorf("%s の大会名が空です", cellText(cells[0]))
		}
		var guidelines *html.Node
		if len(cells) >= 4 {
			guidelines = cells[3]
		}
		link, label := destinationLink(guidelines, sourceURL)
		meets = append(meets, Meet{
			Name:         name,
			StartsOn:     start,
			EndsOn:       end,
			Location:     cellText(cells[2]),
			Prefecture:   "46",
			SeasonYear:   season,
			OrganizerURL: link,
			SourceURL:    sourceURL,
			LinkLabel:    label,
		})
	}
	if len(meets) == 0 {
		return nil, fmt.Errorf("競技日程の行がありません")
	}
	return meets, nil
}

func findSeasonYear(n *html.Node) (int, error) {
	var year int
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if year != 0 || n.Type != html.TextNode {
			for c := n.FirstChild; c != nil && year == 0; c = c.NextSibling {
				walk(c)
			}
			return
		}
		if m := seasonYearPattern.FindStringSubmatch(n.Data); m != nil {
			year, _ = strconv.Atoi(m[1])
		}
	}
	walk(n)
	if year == 0 {
		return 0, fmt.Errorf("競技年度が見つかりません")
	}
	return year, nil
}

func findScheduleTable(n *html.Node) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "table" && tableHeaderIsDate(n) {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return found
}

func tableHeaderIsDate(table *html.Node) bool {
	for _, row := range tableRows(table) {
		for _, cell := range elementChildren(row, "th") {
			if cellText(cell) == "日" {
				return true
			}
		}
	}
	return false
}

func tableRows(table *html.Node) []*html.Node {
	var rows []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
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

func rowCells(row *html.Node) []*html.Node {
	return elementChildren(row, "td")
}

func elementChildren(n *html.Node, tag string) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			out = append(out, c)
		}
	}
	return out
}

func cellText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
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

func parseDate(raw string, season int) (time.Time, *time.Time, error) {
	cleaned := strings.ReplaceAll(raw, "（変更）", "")
	cleaned = strings.ReplaceAll(cleaned, "(変更)", "")
	cleaned = strings.Join(strings.Fields(cleaned), "")
	m := datePattern.FindStringSubmatch(cleaned)
	if m == nil {
		return time.Time{}, nil, fmt.Errorf("日付を解釈できません: %q", raw)
	}
	month, _ := strconv.Atoi(m[1])
	day, _ := strconv.Atoi(m[2])
	endDay := day
	if m[3] != "" {
		endDay, _ = strconv.Atoi(m[3])
	}
	if month < 1 || month > 12 || day < 1 || day > 31 || endDay < day || endDay > 31 {
		return time.Time{}, nil, fmt.Errorf("日付が不正です: %q", raw)
	}
	year := season
	if month < 4 {
		year++
	}
	start := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if start.Month() != time.Month(month) || start.Day() != day {
		return time.Time{}, nil, fmt.Errorf("日付が不正です: %q", raw)
	}
	end := time.Date(year, time.Month(month), endDay, 0, 0, 0, 0, time.UTC)
	if end.Month() != time.Month(month) || end.Day() != endDay {
		return time.Time{}, nil, fmt.Errorf("日付が不正です: %q", raw)
	}
	if end.Equal(start) {
		return start, nil, nil
	}
	return start, &end, nil
}

type anchor struct {
	href string
	text string
}

func destinationLink(cell *html.Node, sourceURL string) (string, string) {
	links := cellAnchors(cell)
	if len(links) == 0 {
		return sourceURL, ""
	}
	link := links[0]
	if isDocument(link.href) || isGuidelinesPath(link.href) {
		return link.href, "要項"
	}
	label := link.text
	if label == "" && cell != nil {
		label = cellText(cell)
	}
	if isHomepageLabel(label) {
		if label == "" {
			label = "ホームページ"
		}
		return link.href, label
	}
	return link.href, "要項"
}

func cellAnchors(cell *html.Node) []anchor {
	if cell == nil {
		return nil
	}
	base, err := url.Parse(KagoshimaPageURL)
	if err != nil {
		return nil
	}
	var links []anchor
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key != "href" {
					continue
				}
				ref, err := url.Parse(strings.TrimSpace(attr.Val))
				if err != nil {
					continue
				}
				resolved := base.ResolveReference(ref)
				if resolved.Scheme != "http" && resolved.Scheme != "https" {
					continue
				}
				links = append(links, anchor{href: resolved.String(), text: cellText(n)})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(cell)
	return links
}

func isDocument(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".xlsm", ".zip", ".csv":
		return true
	default:
		return false
	}
}

func isGuidelinesPath(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	decoded, err := url.PathUnescape(parsed.Path)
	if err != nil {
		decoded = parsed.Path
	}
	lower := strings.ToLower(decoded)
	return strings.Contains(lower, "yoko") || strings.Contains(lower, "youkou") || strings.Contains(decoded, "要項")
}

func isHomepageLabel(label string) bool {
	return strings.Contains(label, "ホームページ") ||
		strings.Contains(label, "陸協") ||
		strings.Contains(label, "体連") ||
		strings.Contains(label, "スポ協")
}
