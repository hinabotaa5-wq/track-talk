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

// regionalIncare maps the number of a federation page on iuau.jp (jigyo/{year}/{n}jigyo.html)
// to its card. The page lists every meet of that federation with its dates and venue.
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
	// The meet code ends in 0101 for the federation's championship; 0102 and later are its side events.
	incareCodePattern  = regexp.MustCompile(`^\d{2}6[1-8]0101$`)
	leadingYearPattern = regexp.MustCompile(`^20\d{2}\s*`)
	dayRangePattern    = regexp.MustCompile(`^(\d{1,2})(?:[~〜～-](\d{1,2}))?$`)
	venueAbbreviations = strings.NewReplacer("（陸）", "陸上競技場", "（運）", "運動公園")
)

// ParseRegionalIncare reads one federation page that is already UTF-8 and returns its
// championship of the fiscal year starting in April of fiscalYear. A row with no
// numeric date (a cancelled meet) is skipped.
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
				// The same meet is listed once per venue.
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
		Name:          leadingYearPattern.ReplaceAllString(cleanText(nodeText(cells[3])), ""),
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

// decodeShiftJIS turns an iuau.jp page, which is served as Shift_JIS, into UTF-8.
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
