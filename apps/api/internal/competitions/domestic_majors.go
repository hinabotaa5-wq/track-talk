package competitions

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	DomesticYouth   = "youth"
	DomesticStudent = "student"
	DomesticOpen    = "open"

	jaafOrigin = "https://www.jaaf.or.jp"
)

// DomesticMajor is one edition of a domestic meet shown on the major-meets page.
type DomesticMajor struct {
	Slug          string
	Year          int
	Name          string
	Category      string
	Venue         string
	StartsOn      time.Time
	EndsOn        time.Time
	ScheduleLabel string
	SiteURL       string
}

// DomesticMeet describes one card. Title picks its entry out of the JAAF monthly list.
// A meet without Title has no machine-readable source and is entered by hand.
type DomesticMeet struct {
	Slug     string
	Category string
	Title    *regexp.Regexp
	Skip     *regexp.Regexp
}

// DomesticMeets is the card order. A regional intercollegiate meet has no page that lists
// its dates, so those eight stay manual until the district federations publish one.
var DomesticMeets = []DomesticMeet{
	{Slug: "nissin-cup", Category: DomesticYouth, Title: regexp.MustCompile(`日清食品カップ.*小学生`)},
	{Slug: "zenchu", Category: DomesticYouth, Title: regexp.MustCompile(`全日本中学校陸上競技選手権大会`)},
	{Slug: "junior-u16-u18", Category: DomesticYouth, Title: regexp.MustCompile(`U18[／/].*U16陸上競技大会$`)},
	{
		Slug:     "junior-u20",
		Category: DomesticYouth,
		Title:    regexp.MustCompile(`U20日本陸上競技選手権大会`),
		Skip:     regexp.MustCompile(`リレー|10000m|競歩|混成`),
	},
	{Slug: "interhigh", Category: DomesticYouth, Title: regexp.MustCompile(`全国高等学校陸上競技対校選手権大会`)},
	{Slug: "incare-hokkaido", Category: DomesticStudent},
	{Slug: "incare-tohoku", Category: DomesticStudent},
	{Slug: "incare-kanto", Category: DomesticStudent},
	{Slug: "incare-hokushinetsu", Category: DomesticStudent},
	{Slug: "incare-tokai", Category: DomesticStudent},
	{Slug: "incare-kansai", Category: DomesticStudent},
	{Slug: "incare-chushikoku", Category: DomesticStudent},
	{Slug: "incare-kyushu", Category: DomesticStudent},
	{Slug: "nihon-incare", Category: DomesticStudent, Title: regexp.MustCompile(`日本学生陸上競技対校選手権大会`)},
	{
		Slug:     "nihon-championships",
		Category: DomesticOpen,
		Title:    regexp.MustCompile(`日本陸上競技選手権大会`),
		Skip:     regexp.MustCompile(`リレー|10000m|競歩|混成|マラソン|駅伝|クロスカントリー`),
	},
	{Slug: "jitsugyodan", Category: DomesticOpen, Title: regexp.MustCompile(`全日本実業団対抗陸上競技選手権大会$`)},
}

// JAAFListEntry is one line of a monthly list on the JAAF competition page.
type JAAFListEntry struct {
	Title string
	Path  string
}

// ParseJAAFList reads the entries of /competition/list/?year=&month=.
func ParseJAAFList(r io.Reader) ([]JAAFListEntry, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	var entries []JAAFListEntry
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" && detailPathPattern.MatchString(attr(n, "href")) {
			if p := firstElement(n, "p"); p != nil {
				entries = append(entries, JAAFListEntry{
					Title: cleanText(nodeText(p)),
					Path:  attr(n, "href"),
				})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return entries, nil
}

// errScheduleUndecided marks a detail page whose 開催日 still reads 調整中.
var errScheduleUndecided = errors.New("開催日が調整中です")

var (
	detailPathPattern    = regexp.MustCompile(`^/competition/detail/\d+/?$`)
	parenthesizedPattern = regexp.MustCompile(`[（(][^）)]*[）)]`)
	wavePattern          = regexp.MustCompile(`[～〜~]`)
)

// ParseJAAFDetail reads one /competition/detail/{id}/ page.
func ParseJAAFDetail(r io.Reader, detailURL string) (DomesticMajor, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return DomesticMajor{}, err
	}
	major := DomesticMajor{SiteURL: detailURL}
	if h := findElement(doc, func(n *html.Node) bool { return n.Data == "h3" && hasClass(n, "middle") }); h != nil {
		major.Name = cleanText(nodeText(h))
	}
	if major.Name == "" {
		return DomesticMajor{}, fmt.Errorf("%s: 大会名がありません", detailURL)
	}

	fields := map[string]string{}
	if dl := findElement(doc, func(n *html.Node) bool { return n.Data == "dl" && hasClass(n, "dlList2") }); dl != nil {
		label := ""
		for c := dl.FirstChild; c != nil; c = c.NextSibling {
			switch {
			case c.Type == html.ElementNode && c.Data == "dt":
				label = cleanText(nodeText(c))
			case c.Type == html.ElementNode && c.Data == "dd" && label != "":
				if _, seen := fields[label]; !seen {
					fields[label] = cleanText(nodeText(c))
				}
			}
		}
	}
	if fields["開催日"] == "調整中" {
		return DomesticMajor{}, errScheduleUndecided
	}
	span := wavePattern.ReplaceAllString(parenthesizedPattern.ReplaceAllString(fields["開催日"], ""), "-")
	start, end, year, precision, ok := parseJapaneseSpan(span)
	if !ok || precision != "day" {
		return DomesticMajor{}, fmt.Errorf("%s: 開催日を読めません: %q", detailURL, fields["開催日"])
	}
	major.Year = year
	major.StartsOn = start
	major.EndsOn = end
	major.ScheduleLabel = scheduleLabel(precision, start, end)
	major.Venue = fields["会場"]

	if files := findElement(doc, func(n *html.Node) bool { return n.Data == "ul" && hasClass(n, "compeFile") }); files != nil {
		for a := firstElement(files, "a"); a != nil; a = nextElement(a, files, "a") {
			if cleanText(nodeText(a)) == "公式HP" && strings.HasPrefix(attr(a, "href"), "http") {
				major.SiteURL = attr(a, "href")
				break
			}
		}
	}
	return major, nil
}

// MatchDomesticMeet returns the card a JAAF list title belongs to.
func MatchDomesticMeet(title string) (DomesticMeet, bool) {
	for _, meet := range DomesticMeets {
		if meet.Title == nil || !meet.Title.MatchString(title) {
			continue
		}
		if meet.Skip != nil && meet.Skip.MatchString(title) {
			continue
		}
		return meet, true
	}
	return DomesticMeet{}, false
}

func findElement(n *html.Node, match func(*html.Node) bool) *html.Node {
	if n.Type == html.ElementNode && match(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findElement(c, match); found != nil {
			return found
		}
	}
	return nil
}

func firstElement(n *html.Node, tag string) *html.Node {
	return findElement(n, func(c *html.Node) bool { return c != n && c.Data == tag })
}

// nextElement walks to the next tag after cur, staying inside root.
func nextElement(cur, root *html.Node, tag string) *html.Node {
	n := cur
	for {
		switch {
		case n.FirstChild != nil:
			n = n.FirstChild
		default:
			for n != root && n.NextSibling == nil {
				n = n.Parent
			}
			if n == root {
				return nil
			}
			n = n.NextSibling
		}
		if n.Type == html.ElementNode && n.Data == tag {
			return n
		}
	}
}
