package competitions

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

const iuauOrigin = "https://www.iuau.jp"

// regionalIncare は、iuau.jp の連盟ページの番号（jigyo/{year}/{n}jigyo.html）をカードに結び付ける。
// そのページには、連盟のすべての大会が日程と会場つきで載っている。
var regionalIncare = map[int]string{
	1: "incare-hokkaido",
	2: "incare-tohoku",
	3: "incare-kanto",
	4: "incare-hokushinetsu",
	5: "incare-tokai",
	6: "incare-kansai",
	7: "incare-chushikoku",
	8: "incare-kyushu",
}

var (
	// 大会コードは、連盟の選手権なら 0101 で終わる。0102 以降は併催の大会。
	incareCodePattern  = regexp.MustCompile(`^\d{2}6[1-8]0101$`)
	leadingYearPattern = regexp.MustCompile(`^20\d{2}\s*`)
	dayRangePattern    = regexp.MustCompile(`^(\d{1,2})(?:[~〜～-](\d{1,2}))?$`)
	venueAbbreviations = strings.NewReplacer("（陸）", "陸上競技場", "（運）", "運動公園")
)

// ParseRegionalIncare は、UTF-8 にした連盟ページを1つ読み、fiscalYear 年の4月から始まる年度の選手権を返す。
// 数字の日付がない行（中止になった大会）は飛ばす。
func ParseRegionalIncare(r io.Reader, fiscalYear int, pageURL string) (*DomesticMajor, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	var found *DomesticMajor
	var venues []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			major, venue, ok := incareRow(n, fiscalYear, pageURL)
			switch {
			case !ok:
			case found == nil:
				found = &major
				venues = []string{venue}
			case found.Name == major.Name && !containsString(venues, venue):
				// 同じ大会が会場ごとに1行ずつ載っている。
				venues = append(venues, venue)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if found != nil {
		found.Venue = strings.Join(venues, "・")
	}
	return found, nil
}

func incareRow(tr *html.Node, fiscalYear int, pageURL string) (DomesticMajor, string, bool) {
	var cells []*html.Node
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "td" {
			cells = append(cells, c)
		}
	}
	if len(cells) < 6 || !incareCodePattern.MatchString(cleanText(nodeText(cells[2]))) {
		return DomesticMajor{}, "", false
	}
	month, err := strconv.Atoi(cleanText(nodeText(cells[0])))
	if err != nil || month < 1 || month > 12 {
		return DomesticMajor{}, "", false
	}
	days := dayRangePattern.FindStringSubmatch(cleanText(nodeText(cells[1])))
	if days == nil {
		return DomesticMajor{}, "", false
	}
	year := fiscalYear
	if month < 4 {
		year++
	}
	startDay, _ := strconv.Atoi(days[1])
	endDay := startDay
	if days[2] != "" {
		endDay, _ = strconv.Atoi(days[2])
	}
	start, ok := civilDate(year, month, startDay)
	if !ok {
		return DomesticMajor{}, "", false
	}
	end, ok := civilDate(year, month, endDay)
	if !ok || end.Before(start) {
		return DomesticMajor{}, "", false
	}
	siteURL := pageURL
	if a := firstElement(cells[5], "a"); a != nil && strings.HasPrefix(attr(a, "href"), "http") {
		siteURL = attr(a, "href")
	}
	return DomesticMajor{
		Year:          year,
		Name:          leadingYearPattern.ReplaceAllString(baseMeetName(nodeText(cells[3])), ""),
		Category:      DomesticStudent,
		StartsOn:      start,
		EndsOn:        end,
		ScheduleLabel: scheduleLabel("day", start, end),
		SiteURL:       siteURL,
	}, venueAbbreviations.Replace(cleanText(nodeText(cells[4]))), true
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// decodeShiftJIS は、Shift_JIS で配信される iuau.jp のページを UTF-8 にする。
func decodeShiftJIS(body []byte) (io.Reader, error) {
	out, _, err := transform.Bytes(japanese.ShiftJIS.NewDecoder(), body)
	if err != nil {
		return nil, fmt.Errorf("Shift_JISを読めません: %w", err)
	}
	return bytes.NewReader(out), nil
}

func iuauPageURL(fiscalYear, federation int) string {
	return fmt.Sprintf("%s/jigyo/%d/%djigyo.html", iuauOrigin, fiscalYear, federation)
}
