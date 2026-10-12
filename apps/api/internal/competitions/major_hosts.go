package competitions

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	MajorWorlds   = "worlds"
	MajorOlympics = "olympics"
)

// MajorHost is one future or past edition taken from a Wikipedia host table.
type MajorHost struct {
	Kind          string
	Year          int
	Name          string
	City          string
	Country       string
	Venue         string
	StartsOn      time.Time
	EndsOn        time.Time
	ScheduleLabel string
	PageURL       string
	CityPageURL   string
	ImageURL      string
}

type wikiLink struct {
	Href  string
	Title string
	Red   bool
}

type wikiCell struct {
	Text   string
	Header bool
	Links  []wikiLink
	Flag   string
}

var (
	hostCountPattern = regexp.MustCompile(`\s*[(（]\d+[)）]`)
	leadingYear      = regexp.MustCompile(`^(\d{4})`)
	datedStart       = regexp.MustCompile(`(\d{4})年\s*(\d{1,2})月\s*(\d{1,2})日`)
	endWithMonth     = regexp.MustCompile(`[-–—]\s*(\d{1,2})月\s*(\d{1,2})日`)
	endDayOnly       = regexp.MustCompile(`[-–—]\s*(\d{1,2})日`)
	yearMonthOnly    = regexp.MustCompile(`(\d{4})年\s*(\d{1,2})月\s*$`)
	yearOnly         = regexp.MustCompile(`(\d{4})年\s*$`)
)

// ParseWorldChampionships reads the 大会一覧 table.
func ParseWorldChampionships(r io.Reader) ([]MajorHost, error) {
	rows, err := uniqueTable(r, "開催日程", "開催都市")
	if err != nil {
		return nil, err
	}
	schedule, ok := headerColumn(rows, "開催日程")
	if !ok {
		return nil, fmt.Errorf("開催日程の列が見つかりません")
	}
	countryCol, _ := headerColumn(rows, "開催国")
	cityCol, ok := headerColumn(rows, "開催都市")
	if !ok {
		return nil, fmt.Errorf("開催都市の列が見つかりません")
	}
	venueCol, _ := headerColumn(rows, "開催会場")
	var hosts []MajorHost
	for _, row := range rows {
		if rowIsHeader(row) || cityCol >= len(row) || schedule >= len(row) {
			continue
		}
		city := cleanPlace(row[cityCol].Text)
		if city == "" || undecided(city) {
			continue
		}
		start, end, year, precision, ok := parseJapaneseSpan(row[schedule].Text)
		if !ok {
			continue
		}
		name, page := hostName(row[0], "世界陸上", "", fmt.Sprintf("%d年世界陸上競技選手権大会", year))
		host := MajorHost{
			Kind: MajorWorlds, Year: year, Name: name, City: city,
			StartsOn: start, EndsOn: end, ScheduleLabel: scheduleLabel(precision, start, end), PageURL: page,
			CityPageURL: firstWikiPage(row[cityCol]),
		}
		if countryCol >= 0 && countryCol < len(row) {
			host.Country = countryName(row[countryCol])
		}
		if venueCol >= 0 && venueCol < len(row) {
			host.Venue = cleanPlace(row[venueCol].Text)
		}
		hosts = append(hosts, host)
	}
	return hosts, nil
}

// ParseSummerOlympics reads the 夏季オリンピック host table.
// The host city is the cell after the year. The session cell also contains a year, so it is not used.
func ParseSummerOlympics(r io.Reader) ([]MajorHost, error) {
	rows, err := uniqueTable(r, "年", "開催都市")
	if err != nil {
		return nil, err
	}
	var hosts []MajorHost
	for _, row := range rows {
		if rowIsHeader(row) {
			continue
		}
		yearIndex := -1
		var year int
		for i, cell := range row {
			if strings.Contains(cell.Text, "総会") {
				continue
			}
			found, ok := leadingYearValue(cell.Text)
			if ok {
				yearIndex = i
				year = found
				break
			}
		}
		if yearIndex < 0 || yearIndex+1 >= len(row) {
			continue
		}
		cityCell := row[yearIndex+1]
		city := cleanPlace(cityCell.Text)
		if city == "" || undecided(city) {
			continue
		}
		start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(year, time.December, 31, 0, 0, 0, 0, time.UTC)
		name, page := hostName(row[yearIndex], "オリンピック", "開催地", fmt.Sprintf("%d年夏季オリンピック", year))
		hosts = append(hosts, MajorHost{
			Kind: MajorOlympics, Year: year, Name: name, City: city,
			Country: cityCell.Flag, StartsOn: start, EndsOn: end,
			ScheduleLabel: scheduleLabel("year", start, end), PageURL: page,
			CityPageURL: firstWikiPage(cityCell),
		})
	}
	return hosts, nil
}

// UpcomingMajorHosts keeps editions that have not ended and returns them by start date.
func UpcomingMajorHosts(hosts []MajorHost, today time.Time) []MajorHost {
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	var out []MajorHost
	for _, host := range hosts {
		if host.EndsOn.IsZero() || host.EndsOn.Before(day) {
			continue
		}
		out = append(out, host)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartsOn.Equal(out[j].StartsOn) {
			return out[i].Name < out[j].Name
		}
		return out[i].StartsOn.Before(out[j].StartsOn)
	})
	return out
}

func uniqueTable(r io.Reader, labels ...string) ([][]wikiCell, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	var found [][][]wikiCell
	eachTable(doc, func(table *html.Node) {
		rows := rowsOf(table)
		if tableHasHeaders(rows, labels...) {
			found = append(found, rows)
		}
	})
	if len(found) != 1 {
		return nil, fmt.Errorf("開催地の表が見つかりません")
	}
	return found[0], nil
}

func tableHasHeaders(rows [][]wikiCell, labels ...string) bool {
	for _, label := range labels {
		if _, ok := headerColumn(rows, label); !ok {
			return false
		}
	}
	return true
}

func headerColumn(rows [][]wikiCell, label string) (int, bool) {
	for _, row := range rows {
		if !rowIsHeader(row) {
			continue
		}
		for i, cell := range row {
			if headerMatch(cell.Text, label) {
				return i, true
			}
		}
	}
	return -1, false
}

func rowIsHeader(row []wikiCell) bool {
	if len(row) == 0 {
		return false
	}
	for _, cell := range row {
		if !cell.Header {
			return false
		}
	}
	return true
}

func headerMatch(text, label string) bool {
	text = strings.TrimSpace(text)
	return text == label || strings.HasPrefix(text, label+" ") || strings.HasPrefix(text, label+"(") || strings.HasPrefix(text, label+"（")
}

func eachTable(n *html.Node, fn func(*html.Node)) {
	if n.Type == html.ElementNode && n.Data == "table" {
		fn(n)
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		eachTable(c, fn)
	}
}

func rowsOf(table *html.Node) [][]wikiCell {
	var rows [][]wikiCell
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n != table && n.Type == html.ElementNode && n.Data == "table" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "tr" {
			var cells []wikiCell
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
					cells = append(cells, readCell(c))
				}
			}
			if len(cells) > 0 {
				rows = append(rows, cells)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(table)
	return rows
}

func readCell(n *html.Node) wikiCell {
	var b strings.Builder
	cell := wikiCell{Header: n.Data == "th"}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "sup", "style", "script":
				return
			case "img":
				alt := attr(n, "alt")
				if cell.Flag == "" && strings.HasSuffix(alt, "の旗") {
					cell.Flag = strings.TrimSuffix(alt, "の旗")
				}
				return
			case "br":
				b.WriteByte(' ')
			case "a":
				href := attr(n, "href")
				cell.Links = append(cell.Links, wikiLink{
					Href:  href,
					Title: attr(n, "title"),
					Red:   strings.Contains(attr(n, "class"), "new") || strings.Contains(href, "redlink=1"),
				})
			}
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	cell.Text = strings.Join(strings.Fields(b.String()), " ")
	return cell
}

func hostName(cell wikiCell, must, avoid, fallback string) (string, string) {
	for _, link := range cell.Links {
		title := strings.TrimSuffix(strings.TrimSpace(link.Title), " (存在しないページ)")
		if must != "" && !strings.Contains(title, must) {
			continue
		}
		if avoid != "" && strings.Contains(title, avoid) {
			continue
		}
		if title == "" {
			continue
		}
		if link.Red {
			return title, ""
		}
		if page, ok := absoluteWiki(link.Href); ok {
			return title, page
		}
		return title, ""
	}
	return fallback, ""
}

func absoluteWiki(href string) (string, bool) {
	if href == "" || strings.Contains(href, "redlink=1") {
		return "", false
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return "", false
	}
	if parsed.Host != "" && parsed.Host != "ja.wikipedia.org" {
		return "", false
	}
	if !strings.HasPrefix(parsed.Path, "/wiki/") {
		return "", false
	}
	title, err := url.PathUnescape(strings.TrimPrefix(parsed.Path, "/wiki/"))
	if err != nil || title == "" || strings.Contains(title, ":") {
		return "", false
	}
	return "https://ja.wikipedia.org/wiki/" + url.PathEscape(title), true
}

func countryName(cell wikiCell) string {
	name := cleanPlace(hostCountPattern.ReplaceAllString(cell.Text, ""))
	if name != "" {
		return name
	}
	return cell.Flag
}

func cleanPlace(text string) string {
	return strings.Trim(strings.TrimSpace(text), "（）()")
}

func undecided(city string) bool {
	switch city {
	case "未定", "未決定", "TBD":
		return true
	default:
		return false
	}
}

func leadingYearValue(text string) (int, bool) {
	match := leadingYear.FindStringSubmatch(strings.TrimSpace(text))
	if match == nil {
		return 0, false
	}
	year, err := strconv.Atoi(match[1])
	if err != nil || year < 1896 || year > 2100 {
		return 0, false
	}
	return year, true
}

func firstWikiPage(cell wikiCell) string {
	for _, link := range cell.Links {
		if link.Red {
			continue
		}
		if page, ok := absoluteWiki(link.Href); ok {
			return page
		}
	}
	return ""
}

func scheduleLabel(precision string, start, end time.Time) string {
	switch precision {
	case "month":
		return fmt.Sprintf("%d年%d月", start.Year(), int(start.Month()))
	case "year":
		return fmt.Sprintf("%d年", start.Year())
	default:
		if start.Equal(end) {
			return fmt.Sprintf("%d年%d月%d日", start.Year(), int(start.Month()), start.Day())
		}
		if start.Year() == end.Year() && start.Month() == end.Month() {
			return fmt.Sprintf("%d年%d月%d日〜%d日", start.Year(), int(start.Month()), start.Day(), end.Day())
		}
		if start.Year() == end.Year() {
			return fmt.Sprintf("%d年%d月%d日〜%d月%d日", start.Year(), int(start.Month()), start.Day(), int(end.Month()), end.Day())
		}
		return fmt.Sprintf("%d年%d月%d日〜%d年%d月%d日", start.Year(), int(start.Month()), start.Day(), end.Year(), int(end.Month()), end.Day())
	}
}

// MajorHostsForStore keeps the previous rows when a parse finds nothing upcoming.
func MajorHostsForStore(parsed []MajorHost) (next []MajorHost, replace bool) {
	if len(parsed) == 0 {
		return nil, false
	}
	return parsed, true
}

func parseJapaneseSpan(text string) (start, end time.Time, year int, precision string, ok bool) {
	text = strings.TrimSpace(text)
	if match := datedStart.FindStringSubmatch(text); match != nil {
		year, _ = strconv.Atoi(match[1])
		month, _ := strconv.Atoi(match[2])
		day, _ := strconv.Atoi(match[3])
		start, ok = civilDate(year, month, day)
		if !ok {
			return time.Time{}, time.Time{}, 0, "", false
		}
		if endMatch := endWithMonth.FindStringSubmatch(text); endMatch != nil {
			endMonth, _ := strconv.Atoi(endMatch[1])
			endDay, _ := strconv.Atoi(endMatch[2])
			endYear := year
			if endMonth < month {
				endYear++
			}
			end, ok = civilDate(endYear, endMonth, endDay)
			if !ok {
				return time.Time{}, time.Time{}, 0, "", false
			}
			return start, end, year, "day", true
		}
		if endMatch := endDayOnly.FindStringSubmatch(text); endMatch != nil {
			endDay, _ := strconv.Atoi(endMatch[1])
			end, ok = civilDate(year, month, endDay)
			if !ok {
				return time.Time{}, time.Time{}, 0, "", false
			}
			return start, end, year, "day", true
		}
		return start, start, year, "day", true
	}
	if match := yearMonthOnly.FindStringSubmatch(text); match != nil {
		year, _ = strconv.Atoi(match[1])
		month, _ := strconv.Atoi(match[2])
		start, ok = civilDate(year, month, 1)
		if !ok {
			return time.Time{}, time.Time{}, 0, "", false
		}
		end = start.AddDate(0, 1, -1)
		return start, end, year, "month", true
	}
	if match := yearOnly.FindStringSubmatch(text); match != nil {
		year, _ = strconv.Atoi(match[1])
		start = time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(year, time.December, 31, 0, 0, 0, 0, time.UTC)
		return start, end, year, "year", true
	}
	return time.Time{}, time.Time{}, 0, "", false
}

func civilDate(year, month, day int) (time.Time, bool) {
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, false
	}
	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if date.Year() != year || int(date.Month()) != month || date.Day() != day {
		return time.Time{}, false
	}
	return date, true
}
