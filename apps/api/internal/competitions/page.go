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

// Page は、協会のホームページ1つから取った日程の文字とリンク。
type Page struct {
	SeasonYear int
	Body       string
	Links      []LinkRef
	rows       []row
	plainText  bool
	// 日程が PDF でしか公開されていないときは、Body の代わりに PDFs を入れる。
	PDFs [][]byte
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
	year, ok := yearFromURL(base)
	if !ok {
		year, ok = findSeasonYear(doc)
	}
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
		body, textLinks := plainTextPage(doc, base)
		if !looksLikeSchedule(body) {
			return Page{}, fmt.Errorf("競技日程の表が見つかりません")
		}
		// そういうページは前のシーズンにもリンクしていることが多いので、書かれている中でいちばん新しいシーズンを取る。
		if newest, ok := newestSeasonYear(doc); ok {
			year = newest
		}
		return Page{SeasonYear: year, Body: body, Links: textLinks, plainText: true}, nil
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

var (
	urlQueryYearPattern = regexp.MustCompile(`(?:^|[?&])(?:y|year)=(20\d{2})`)
	urlPathYearPattern  = regexp.MustCompile(`(?:/|_|-)(20\d{2})[a-zA-Z0-9_.-]*(?:/|$)`)
)

func yearFromURL(u *url.URL) (int, bool) {
	if u == nil {
		return 0, false
	}
	for _, key := range []string{"y", "year", "season"} {
		if val := u.Query().Get(key); val != "" {
			if y, err := strconv.Atoi(val); err == nil && y >= 2020 && y <= seasonYearCap() {
				// 月ごとのページ（?m=1&y=2027 など）の y は暦年。1〜3月は前の年の競技年度に入る。
				if key != "season" {
					if m, err := strconv.Atoi(u.Query().Get("m")); err == nil && m >= 1 && m <= 3 {
						y--
					}
				}
				return y, true
			}
		}
	}
	if match := urlQueryYearPattern.FindStringSubmatch(u.RawQuery); match != nil {
		if y, err := strconv.Atoi(match[1]); err == nil && y >= 2020 && y <= seasonYearCap() {
			return y, true
		}
	}
	if match := urlPathYearPattern.FindStringSubmatch(u.Path); match != nil {
		if y, err := strconv.Atoi(match[1]); err == nil && y >= 2020 && y <= seasonYearCap() {
			return y, true
		}
	}
	return 0, false
}

func findSeasonYear(doc *html.Node) (int, bool) {
	if newest, ok := newestSeasonYear(doc); ok {
		return newest, true
	}
	text := normalizeDigits(nodeText(doc))
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:R|令和)\s*\d{1,2}\s*[（(]\s*(20\d{2})\s*[）)]\s*年度`),
		regexp.MustCompile(`(20\d{2})年競技`),
		regexp.MustCompile(`(20\d{2})年度`),
		seasonYearPattern,
		regexp.MustCompile(`(20\d{2})[\s　]*(?:[^\s　\d]{1,15}[\s　]*)?(?:競技日程|競技会情報)`),
	}
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(text)
		if match == nil {
			continue
		}
		year, err := strconv.Atoi(match[1])
		if err == nil && year >= 1990 && year <= seasonYearCap() {
			return year, true
		}
	}
	// Some pages only say 令和８年度 with no Western year. Reiwa 1 is 2019, so the season starts in 2018+N.
	newest := 0
	for _, match := range eraSeasonPattern.FindAllStringSubmatch(text, -1) {
		if n, err := strconv.Atoi(match[1]); err == nil && n >= 1 && 2018+n <= seasonYearCap() && 2018+n > newest {
			newest = 2018 + n
		}
	}
	if newest > 0 {
		return newest, true
	}
	return 0, false
}

var eraSeasonPattern = regexp.MustCompile(`(?:R|令和)\s*(\d{1,2})\s*年度`)

var (
	headingYearPattern       = regexp.MustCompile(`(20\d{2})年度`)
	monthHeadingPattern      = regexp.MustCompile(`(?:(20\d{2})年)?\s*(0?[1-9]|1[0-2])月`)
	digitMonthPattern        = regexp.MustCompile(`\d月`)
	clockPattern             = regexp.MustCompile(`\d{1,2}:\d{2}\s*[～〜~]?`)
	headlessFirstCellPattern = regexp.MustCompile(`^\d{1,2}(?:[\s,、・/～〜-]\d{1,2})*(?:日)?$`)
	weekdayPattern           = regexp.MustCompile(`^[月火水木金土日祝・,、/\s()（）]+$`)
	yearHeaderPattern        = regexp.MustCompile(`^20\d{2}年?$`)
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
			if isCalendarWidget(n) {
				return
			}
			if !hasNestedTable(n) {
				headers, headerAt := tableHeaders(n)
				if isScheduleHeader(headers) && (section == 0 || section == season) {
					tblMonth := tableLeadingMonth(n, headerAt)
					if tblMonth == "" {
						tblMonth = month
					}
					tables = append(tables, foundTable{node: n, month: tblMonth})
					return
				}
			}
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

func hasNestedTable(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "table" {
			return true
		}
		if hasNestedTable(c) {
			return true
		}
	}
	return false
}

func tableLeadingMonth(table *html.Node, headerAt int) string {
	if headerAt <= 0 {
		return ""
	}
	rows := directRows(table)
	for i := 0; i < headerAt && i < len(rows); i++ {
		text := normalizeDigits(cellText(rows[i]))
		monthMatch := monthHeadingPattern.FindStringSubmatch(text)
		if monthMatch != nil {
			yearMatch := headingYearPattern.FindStringSubmatch(text)
			if monthMatch[1] == "" && yearMatch != nil {
				return yearMatch[1] + "年" + monthMatch[2] + "月"
			}
			return monthMatch[0]
		}
	}
	return ""
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
	if !isDateHeader(header) {
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

// scheduleListRows は、大会を表ではなく項目で並べているホームページを読む。
// そのページに書かれている名前・日付・会場だけを残す。
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
			if node == nil || seen[node] {
				continue
			}
			seen[node] = true
			header := ""
			if col < len(headers) {
				header = headers[col]
			}
			if node.Data == "th" && !isDateHeader(header) {
				continue
			}
			text := cellText(node)
			if isDateHeader(header) {
				text = cleanDateText(withMonth(header, normalizeDigits(text), month))
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
	compact := compactHeader(header)
	return header == "日" || header == "月" || header == "月日" || header == "期日" || isDayWithWeekday(header) || strings.Contains(header, "開催日") || strings.Contains(header, "日程") || strings.Contains(header, "日／曜") || strings.Contains(header, "日/曜") || strings.Contains(compact, "日/曜") || yearHeaderPattern.MatchString(header)
}

// isDayWithWeekday matches a column titled 日（曜）or 日(曜).
func isDayWithWeekday(header string) bool {
	return strings.HasPrefix(header, "日（") || strings.HasPrefix(header, "日(")
}

func cleanDateText(text string) string {
	return strings.TrimSpace(clockPattern.ReplaceAllString(text, ""))
}

func isNameHeader(header string) bool {
	if strings.Contains(header, "要項") {
		return false
	}
	compact := compactHeader(header)
	return strings.Contains(header, "競技会") || strings.Contains(header, "行事") || strings.Contains(header, "大会") || strings.Contains(header, "主催") || strings.Contains(compact, "内容")
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

func isHeadlessDataRow(firstCellText string, remainingCells int, secondCellText string) bool {
	firstCellText = strings.TrimSpace(normalizeDigits(firstCellText))
	if !headlessFirstCellPattern.MatchString(firstCellText) {
		return false
	}
	if remainingCells >= 1 && weekdayPattern.MatchString(strings.TrimSpace(secondCellText)) {
		return true
	}
	return remainingCells >= 2
}

func synthesizedHeaders(cols int) []string {
	switch cols {
	case 3:
		return []string{"日", "競技会", "会場"}
	case 4:
		return []string{"日", "曜日", "競技会", "会場"}
	case 5:
		return []string{"日", "曜日", "競技会", "会場", "備考"}
	default:
		headers := make([]string, cols)
		for i := range headers {
			switch i {
			case 0:
				headers[i] = "日"
			case 1:
				headers[i] = "曜日"
			case 2:
				headers[i] = "競技会"
			case 3:
				headers[i] = "会場"
			default:
				headers[i] = fmt.Sprintf("備考%d", i-3)
			}
		}
		return headers
	}
}

func inThead(n *html.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && p.Data == "thead" {
			return true
		}
	}
	return false
}

func gridHeaders(grid [][]*html.Node) ([]string, int) {
	for i, line := range grid {
		inTh := false
		for _, node := range line {
			if node != nil && inThead(node) {
				inTh = true
				break
			}
		}
		if inTh {
			labels := make([]string, len(line))
			for col, node := range line {
				if node != nil {
					labels[col] = compactHeader(cellText(node))
				}
			}
			return labels, i
		}
	}
	for i, line := range grid {
		thCount := 0
		total := 0
		for _, node := range line {
			if node == nil {
				continue
			}
			total++
			if node.Data == "th" {
				thCount++
			}
		}
		if thCount > 0 && thCount*2 >= total {
			labels := make([]string, len(line))
			for col, node := range line {
				if node != nil {
					labels[col] = compactHeader(cellText(node))
				}
			}
			return labels, i
		}
	}
	if len(grid) == 0 {
		return nil, 0
	}
	limit := len(grid)
	if limit > 4 {
		limit = 4
	}
	for i := 0; i < limit; i++ {
		line := grid[i]
		if i == 0 && len(line) > 0 && line[0] != nil {
			second := ""
			if len(line) > 1 && line[1] != nil {
				second = cellText(line[1])
			}
			if isHeadlessDataRow(cellText(line[0]), len(line)-1, second) {
				return synthesizedHeaders(len(line)), -1
			}
		}
		candidate := make([]string, len(line))
		for col, node := range line {
			if node != nil {
				candidate[col] = compactHeader(cellText(node))
			}
		}
		if isScheduleHeader(candidate) {
			return candidate, i
		}
	}
	firstLine := grid[0]
	labels := make([]string, len(firstLine))
	for col, node := range firstLine {
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
		// ブラウザと同じく上限で切る（HTML の仕様で colspan は1000、rowspan は65534）。
		// 切らないと、巨大な値のセルが1つあるだけで表の展開がメモリを使い切る。
		if limit := map[string]int{"colspan": 1000, "rowspan": 65534}[strings.ToLower(key)]; limit > 0 && value > limit {
			return limit
		}
		return value
	}
	return 1
}

func isCalendarWidget(n *html.Node) bool {
	for _, attr := range n.Attr {
		if attr.Key == "id" && attr.Val == "wp-calendar" {
			return true
		}
		if attr.Key == "class" && (strings.Contains(attr.Val, "wp-calendar") || strings.Contains(attr.Val, "MonthIdouTable") || strings.Contains(attr.Val, "YearListTable")) {
			return true
		}
		if attr.Key == "summary" && strings.Contains(attr.Val, "月操作") {
			return true
		}
	}
	return false
}

func isDayOfWeekGrid(headers []string) bool {
	days := map[string]bool{"月": true, "火": true, "水": true, "木": true, "金": true, "土": true, "日": true}
	count := 0
	for _, h := range headers {
		if days[strings.TrimSpace(h)] {
			count++
		}
	}
	return count >= 5
}

func isScheduleHeader(headers []string) bool {
	if len(headers) < 2 {
		return false
	}
	if isDayOfWeekGrid(headers) {
		return false
	}
	for _, header := range headers {
		header = strings.TrimSpace(header)
		if len([]rune(header)) > 25 {
			continue
		}
		if isDateHeader(header) || strings.Contains(header, "要項") {
			return true
		}
	}
	return false
}

func tableHeaders(table *html.Node) ([]string, int) {
	rows := directRows(table)
	for i, row := range rows {
		if !inThead(row) {
			continue
		}
		var cells []*html.Node
		for c := row.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.Data == "th" || c.Data == "td") {
				cells = append(cells, c)
			}
		}
		if len(cells) > 0 {
			headers := make([]string, len(cells))
			for j, cell := range cells {
				headers[j] = compactHeader(cellText(cell))
			}
			return headers, i
		}
	}
	for i, row := range rows {
		thCount := 0
		total := 0
		for c := row.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.Data == "th" || c.Data == "td") {
				total++
				if c.Data == "th" {
					thCount++
				}
			}
		}
		if thCount > 0 && thCount*2 >= total {
			var headers []string
			for c := row.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "th" || c.Data == "td") {
					headers = append(headers, compactHeader(cellText(c)))
				}
			}
			return headers, i
		}
	}
	if len(rows) == 0 {
		return nil, 0
	}
	limit := len(rows)
	if limit > 4 {
		limit = 4
	}
	for i := 0; i < limit; i++ {
		cells := elementChildren(rows[i], "td")
		if len(cells) == 0 {
			continue
		}
		if i == 0 {
			second := ""
			if len(cells) > 1 {
				second = cellText(cells[1])
			}
			if isHeadlessDataRow(cellText(cells[0]), len(cells)-1, second) {
				return synthesizedHeaders(len(cells)), -1
			}
		}
		candidate := make([]string, len(cells))
		for j, cell := range cells {
			candidate[j] = compactHeader(cellText(cell))
		}
		if isScheduleHeader(candidate) {
			return candidate, i
		}
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
