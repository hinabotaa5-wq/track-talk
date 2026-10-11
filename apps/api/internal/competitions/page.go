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
	for _, table := range scheduleTables(doc, year) {
		parsed, parsedLinks := rowsFromTable(table.node, table.month, base)
		rows = append(rows, parsed...)
		links = append(links, parsedLinks...)
	}
	if len(rows) == 0 {
		rows = scheduleListRows(doc)
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

var (
	headingYearPattern  = regexp.MustCompile(`(20\d{2})年度`)
	monthHeadingPattern = regexp.MustCompile(`(20\d{2})年(0?[1-9]|1[0-2])月`)
	digitMonthPattern   = regexp.MustCompile(`\d月`)
)

type foundTable struct {
	node  *html.Node
	month string
}

func scheduleTables(doc *html.Node, season int) []foundTable {
	var tables []foundTable
	section := 0
	month := ""
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			headers, _ := tableHeaders(n)
			if isScheduleHeader(headers) && (section == 0 || section == season) {
				tables = append(tables, foundTable{node: n, month: month})
			}
			return
		}
		if y, ok := headingSeason(n); ok {
			section = y
		}
		if label, ok := headingMonth(n); ok {
			month = label
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return tables
}

func headingMonth(n *html.Node) (string, bool) {
	if n.Type != html.ElementNode {
		return "", false
	}
	switch n.Data {
	case "h1", "h2", "h3", "h4", "h5", "h6", "p", "b", "strong":
	default:
		return "", false
	}
	text := strings.TrimSpace(normalizeDigits(cellText(n)))
	if text == "" || len([]rune(text)) > 20 {
		return "", false
	}
	match := monthHeadingPattern.FindStringSubmatch(text)
	if match == nil {
		return "", false
	}
	return match[0], true
}

func withMonth(header, text, month string) string {
	text = strings.TrimSpace(text)
	if month == "" || text == "" || digitMonthPattern.MatchString(text) {
		return text
	}
	if header != "日" && !strings.Contains(header, "開催日") {
		return text
	}
	return month + " " + text
}

func headingSeason(n *html.Node) (int, bool) {
	if n.Type != html.ElementNode {
		return 0, false
	}
	switch n.Data {
	case "h1", "h2", "h3", "h4", "h5", "h6", "p", "b", "strong", "span":
	default:
		return 0, false
	}
	text := strings.TrimSpace(normalizeDigits(cellText(n)))
	if text == "" || len([]rune(text)) > 30 {
		return 0, false
	}
	match := headingYearPattern.FindStringSubmatch(text)
	if match == nil {
		return 0, false
	}
	year, err := strconv.Atoi(match[1])
	if err != nil || year < 1990 || year > 2100 {
		return 0, false
	}
	return year, true
}

var (
	listHeldPattern  = regexp.MustCompile(`開催\s*[:：]\s*(\d{4}年\d{1,2}月\d{1,2}日)`)
	listVenuePattern = regexp.MustCompile(`会場\s*[:：]\s*(\S+)`)
)

// scheduleListRows reads a homepage that lists meets as items instead of a table.
// Only the name, date, and venue written on that page are kept.
func scheduleListRows(doc *html.Node) []row {
	var rows []row
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" {
			text := normalizeDigits(cellText(n))
			held := listHeldPattern.FindStringSubmatch(text)
			name := strongText(n)
			if held != nil && name != "" {
				venue := ""
				if match := listVenuePattern.FindStringSubmatch(text); match != nil {
					venue = match[1]
				}
				rows = append(rows, row{Cells: []cell{
					{Header: "開催日", Text: held[1]},
					{Header: "競技会", Text: name},
					{Header: "会場", Text: venue},
				}})
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return rows
}

func strongText(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "strong" {
		return cellText(n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if text := strongText(c); text != "" {
			return text
		}
	}
	return ""
}

func rowsFromTable(table *html.Node, month string, base *url.URL) ([]row, []LinkRef) {
	grid := layoutTable(table)
	headers, headerAt := gridHeaders(grid)
	if !isScheduleHeader(headers) {
		return nil, nil
	}
	named := false
	for _, header := range headers {
		if isNameHeader(header) {
			named = true
			break
		}
	}
	var rows []row
	var links []LinkRef
	for i := headerAt + 1; i < len(grid); i++ {
		var ordered []placed
		seen := map[*html.Node]bool{}
		for col, node := range grid[i] {
			if node == nil || node.Data == "th" || seen[node] {
				continue
			}
			seen[node] = true
			header := ""
			if col < len(headers) {
				header = headers[col]
			}
			text := cellText(node)
			if isDateHeader(header) {
				text = withMonth(header, normalizeDigits(text), month)
			}
			parsedLinks := cellLinks(node, base, header)
			links = append(links, parsedLinks...)
			ordered = append(ordered, placed{
				cell: cell{Header: header, Text: text, Links: parsedLinks},
				node: node,
			})
		}
		if len(ordered) == 0 {
			continue
		}
		if !named {
			cells := make([]cell, len(ordered))
			for j, item := range ordered {
				cells[j] = item.cell
			}
			rows = append(rows, row{Cells: cells})
			continue
		}
		var shared []cell
		for _, item := range ordered {
			if isNameHeader(item.cell.Header) {
				continue
			}
			shared = append(shared, item.cell)
		}
		for _, item := range ordered {
			if !isNameHeader(item.cell.Header) {
				continue
			}
			lines := cellLines(item.node)
			for _, line := range lines {
				copied := append([]cell(nil), shared...)
				name := item.cell
				name.Text = line
				if len(lines) > 1 {
					name.Links = nil
				}
				copied = append(copied, name)
				rows = append(rows, row{Cells: copied})
			}
		}
	}
	return rows, links
}

type placed struct {
	cell cell
	node *html.Node
}

func isDateHeader(header string) bool {
	return header == "日" || header == "月" || strings.Contains(header, "開催日") || strings.Contains(header, "日程")
}

func isNameHeader(header string) bool {
	if strings.Contains(header, "要項") {
		return false
	}
	return strings.Contains(header, "競技会") || strings.Contains(header, "行事") || strings.Contains(header, "大会") || strings.Contains(header, "主催")
}

func cellLines(n *html.Node) []string {
	var lines []string
	var b strings.Builder
	flush := func() {
		text := strings.Join(strings.Fields(b.String()), " ")
		b.Reset()
		if text != "" {
			lines = append(lines, text)
		}
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && isStruck(n) {
			return
		}
		if n.Type == html.ElementNode && n.Data == "br" {
			flush()
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	flush()
	return lines
}

func layoutTable(table *html.Node) [][]*html.Node {
	type pending struct {
		node *html.Node
		rows int
	}
	open := map[int]pending{}
	var grid [][]*html.Node
	width := 0
	for _, tr := range directRows(table) {
		row := []*html.Node{}
		col := 0
		ensure := func(c int) {
			for len(row) <= c {
				row = append(row, nil)
			}
		}
		fillOpen := func() {
			for {
				item, ok := open[col]
				if !ok {
					return
				}
				ensure(col)
				row[col] = item.node
				item.rows--
				if item.rows == 0 {
					delete(open, col)
				} else {
					open[col] = item
				}
				col++
			}
		}
		for _, cell := range rowCells(tr) {
			fillOpen()
			cols := spanAttr(cell, "colspan")
			rows := spanAttr(cell, "rowspan")
			for k := 0; k < cols; k++ {
				ensure(col)
				row[col] = cell
				if rows > 1 {
					open[col] = pending{node: cell, rows: rows - 1}
				}
				col++
			}
		}
		fillOpen()
		if col > width {
			width = col
		}
		grid = append(grid, row)
	}
	for i := range grid {
		for len(grid[i]) < width {
			grid[i] = append(grid[i], nil)
		}
	}
	return grid
}

func gridHeaders(grid [][]*html.Node) ([]string, int) {
	for i, line := range grid {
		labels := make([]string, len(line))
		found := false
		for col, node := range line {
			if node == nil || node.Data != "th" {
				continue
			}
			found = true
			labels[col] = compactHeader(cellText(node))
		}
		if found {
			return labels, i
		}
	}
	if len(grid) == 0 {
		return nil, 0
	}
	labels := make([]string, len(grid[0]))
	for col, node := range grid[0] {
		if node != nil {
			labels[col] = compactHeader(cellText(node))
		}
	}
	return labels, 0
}

func rowCells(tr *html.Node) []*html.Node {
	var out []*html.Node
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
			out = append(out, c)
		}
	}
	return out
}

func spanAttr(n *html.Node, key string) int {
	for _, attr := range n.Attr {
		if !strings.EqualFold(attr.Key, key) {
			continue
		}
		value, err := strconv.Atoi(strings.TrimSpace(attr.Val))
		if err != nil || value < 1 {
			return 1
		}
		return value
	}
	return 1
}

func isScheduleHeader(headers []string) bool {
	for _, header := range headers {
		header = strings.TrimSpace(header)
		if header == "日" || strings.Contains(header, "開催日") || strings.Contains(header, "要項") || strings.Contains(header, "日程") {
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
			headers[j] = compactHeader(cellText(cell))
		}
		return headers, i
	}
	if len(rows) == 0 {
		return nil, 0
	}
	cells := elementChildren(rows[0], "td")
	headers := make([]string, len(cells))
	for i, cell := range cells {
		headers[i] = compactHeader(cellText(cell))
	}
	return headers, 0
}

func compactHeader(header string) string {
	return strings.Join(strings.Fields(header), "")
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
