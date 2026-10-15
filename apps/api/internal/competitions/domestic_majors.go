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
	// DomesticDistance is the ekiden and marathon meets of the off-season.
	DomesticDistance = "distance"

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
// A meet without Title is not on the JAAF list; the regional intercollegiate meets come
// from the federation pages on iuau.jp instead (see regionalIncare).
type DomesticMeet struct {
	Slug     string
	Category string
	Title    *regexp.Regexp
	Skip     *regexp.Regexp
}

// DomesticMeets is the card order.
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
	{Slug: "izumo-ekiden", Category: DomesticDistance, Title: regexp.MustCompile(`出雲全日本大学選抜駅伝`)},
	{Slug: "univ-women-ekiden", Category: DomesticDistance, Title: regexp.MustCompile(`全日本大学女子駅伝対校選手権大会`)},
	{Slug: "univ-ekiden", Category: DomesticDistance, Title: regexp.MustCompile(`全日本大学駅伝対校選手権大会`)},
	{Slug: "queens-ekiden", Category: DomesticDistance, Title: regexp.MustCompile(`全日本実業団対抗女子駅伝競走大会`)},
	{Slug: "junior-high-ekiden", Category: DomesticDistance, Title: regexp.MustCompile(`全国中学駅伝大会`)},
	{Slug: "high-school-ekiden", Category: DomesticDistance, Title: regexp.MustCompile(`全国高等学校駅伝競走大会`)},
	{Slug: "fukuoka-marathon", Category: DomesticDistance, Title: regexp.MustCompile(`福岡国際マラソン`)},
	{Slug: "fujisan-ekiden", Category: DomesticDistance, Title: regexp.MustCompile(`全日本大学女子選抜駅伝`)},
	{Slug: "new-year-ekiden", Category: DomesticDistance, Title: regexp.MustCompile(`全日本実業団対抗駅伝競走大会`)},
	{Slug: "prefectural-women-ekiden", Category: DomesticDistance, Title: regexp.MustCompile(`全国都道府県対抗女子駅伝`)},
	{Slug: "prefectural-men-ekiden", Category: DomesticDistance, Title: regexp.MustCompile(`全国都道府県対抗男子駅伝`)},
	{Slug: "osaka-women-marathon", Category: DomesticDistance, Title: regexp.MustCompile(`大阪国際女子マラソン`)},
	{Slug: "beppu-oita-marathon", Category: DomesticDistance, Title: regexp.MustCompile(`別府大分毎日マラソン`)},
	{Slug: "osaka-marathon", Category: DomesticDistance, Title: regexp.MustCompile(`大阪マラソン`)},
	{Slug: "tokyo-marathon", Category: DomesticDistance, Title: regexp.MustCompile(`東京マラソン\d{4}`)},
	{Slug: "nagoya-women-marathon", Category: DomesticDistance, Title: regexp.MustCompile(`名古屋ウィメンズマラソン`)},
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
	yearlessDatePattern  = regexp.MustCompile(`^\d{1,2}月\d{1,2}日`)
)

// ParseJAAFDetail reads one /competition/detail/{id}/ page. listedYear is the year of the monthly
// list that linked to it, used when the 開催日 has no year (「12月30日（水）予定」). Zero leaves it unread.
func ParseJAAFDetail(r io.Reader, detailURL string, listedYear int) (DomesticMajor, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return DomesticMajor{}, err
	}
	major := DomesticMajor{SiteURL: detailURL}
	if h := findElement(doc, func(n *html.Node) bool { return n.Data == "h3" && hasClass(n, "middle") }); h != nil {
		major.Name = baseMeetName(nodeText(h))
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
	span = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(span), "予定"))
	if listedYear != 0 && yearlessDatePattern.MatchString(span) {
		span = fmt.Sprintf("%d年%s", listedYear, span)
	}
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

var (
	alsoPattern      = regexp.MustCompile(`\s*兼\s.*$`)
	mgcPrefixPattern = regexp.MustCompile(`^\d{4}-\d{4}\s*MGCシリーズ\s*`)
	mgcSuffixPattern = regexp.MustCompile(`\s*[～〜][^～〜]*MGCシリーズ[^～〜]*[～〜]$`)
)

// baseMeetName drops the "兼 …" tail that JAAF adds when a meet doubles as a team-selection meet,
// and the MGC series labels around marathon names.
func baseMeetName(raw string) string {
	name := alsoPattern.ReplaceAllString(cleanText(raw), "")
	name = mgcPrefixPattern.ReplaceAllString(name, "")
	return mgcSuffixPattern.ReplaceAllString(name, "")
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
