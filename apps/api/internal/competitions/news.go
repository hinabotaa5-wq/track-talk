package competitions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// NewsItem は、Gemini が返した、県の協会のニュース一覧にあるお知らせ1件。
type NewsItem struct {
	Title      string `json:"title"`
	ArticleURL string `json:"article_url"`
	EventDate  string `json:"event_date"`
	Venue      string `json:"venue"`
}

// KnownMeet は、年間の日程表から入った大会。ニュースから大会を増やすことはなく、これらに要項を付けるだけ。
type KnownMeet struct {
	Key      string
	Name     string
	StartsOn time.Time
	EndsOn   *time.Time
	Location string
}

// NewsMatch は、ニュース1件と、それが知らせている年間の大会1件の組。
type NewsMatch struct {
	Item NewsItem
	Meet KnownMeet
}

// FetchNewsList は、ニュース一覧のページを文字の行として読み、リンクのあとにその URL を付ける。
func FetchNewsList(ctx context.Context, pageURL string, seasonYear int) (Page, error) {
	res, finalURL, err := get(ctx, pageURL)
	if err != nil {
		return Page{}, err
	}
	defer res.Body.Close()
	reader, err := charset.NewReader(res.Body, res.Header.Get("Content-Type"))
	if err != nil {
		return Page{}, err
	}
	doc, err := html.Parse(io.LimitReader(reader, 8<<20))
	if err != nil {
		return Page{}, err
	}
	base, err := url.Parse(finalURL)
	if err != nil {
		return Page{}, err
	}
	body, links := plainTextPage(doc, base)
	if len(links) == 0 {
		return Page{}, fmt.Errorf("%s にリンクがありません", pageURL)
	}
	return Page{SeasonYear: seasonYear, Body: body, Links: links, plainText: true}, nil
}

// ExtractNews は、ニュース一覧のページにある大会のお知らせを Gemini に聞く。リクエストは1回。
func (c Client) ExtractNews(ctx context.Context, page Page) ([]NewsItem, error) {
	text, err := c.generate(ctx, []map[string]any{{"text": newsPrompt(page)}})
	if err != nil {
		return nil, err
	}
	items, err := parseNews(text)
	if err != nil {
		return nil, err
	}
	// Gemini が作り出したリンクは開かない。
	var kept []NewsItem
	for _, item := range items {
		if linkOnPage(page, item.ArticleURL) && strings.TrimSpace(item.Title) != "" {
			kept = append(kept, item)
		}
	}
	return kept, nil
}

func newsPrompt(page Page) string {
	return fmt.Sprintf(`この陸上競技協会の新着一覧から、大会の開催案内や要項の掲載を知らせる記事だけをJSONで返してください。形式は {"news":[{"title":"","article_url":"","event_date":"","venue":""}]} です。
title は記事の題名、article_url は題名のリンク先のURLで、本文に書かれているURLだけを使ってください。リンクが無い記事は返さないでください。
event_date は大会の開催日が題名や一覧に書かれているときだけ YYYY-MM-DD にしてください。競技年度は%d年で、4月から12月はその年、1月から3月は翌年です。記事の掲載日や更新日は event_date に入れないでください。開催日が無ければ空文字です。
venue は会場が書かれているときだけ入れてください。無ければ空文字です。
大会の結果、講習会、研修、理事会、会議、練習会、合宿、審判、表彰、一般のお知らせは返さないでください。

%s`, page.SeasonYear, page.Body)
}

func parseNews(raw string) ([]NewsItem, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	var wrapped struct {
		News []NewsItem `json:"news"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapped); err == nil && wrapped.News != nil {
		return wrapped.News, nil
	}
	var list []NewsItem
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil, fmt.Errorf("gemini json: %w", err)
	}
	return list, nil
}

func linkOnPage(page Page, raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	for _, link := range page.Links {
		if sameURL(link.URL, raw) {
			return true
		}
	}
	return false
}

// nonMeetNews は、大会の開催についてではないお知らせ。Gemini にも飛ばすよう伝えているが、これは取りこぼしの受け皿。
var nonMeetNews = []string{"結果", "講習会", "研修", "理事会", "会議", "練習会", "合宿", "表彰", "審判"}

// MatchNews は、ニュースごとに、それが知らせている年間の大会を1つだけ組み合わせる。どの大会にも合わないものや、
// 2つ以上に合うものは捨てる。間違った要項のリンクは、ページが代わりに出す協会の一覧より悪いから。
func MatchNews(items []NewsItem, meets []KnownMeet) []NewsMatch {
	var matches []NewsMatch
	claimed := map[string]bool{}
	for _, item := range items {
		if isNonMeetNews(item.Title) {
			continue
		}
		meet, ok := matchMeet(item, meets)
		if !ok || claimed[meet.Key] {
			continue
		}
		claimed[meet.Key] = true
		matches = append(matches, NewsMatch{Item: item, Meet: meet})
	}
	return matches
}

func isNonMeetNews(title string) bool {
	for _, word := range nonMeetNews {
		if strings.Contains(title, word) {
			return true
		}
	}
	return false
}

// minContainedName は、年間の大会名がニュースのタイトルの中に含まれるだけのとき、数に入れるのに必要な名前の長さ。
const minContainedName = 6

func matchMeet(item NewsItem, meets []KnownMeet) (KnownMeet, bool) {
	news := normalizeMeetName(item.Title)
	if news == "" {
		return KnownMeet{}, false
	}
	var exact, loose []KnownMeet
	for _, meet := range meets {
		name := normalizeMeetName(meet.Name)
		switch {
		case name == "":
		case name == news:
			exact = append(exact, meet)
		case strings.Contains(name, news):
			// A short news name such as 県中学陸上 only counts when the annual name contains it.
			loose = append(loose, meet)
		case len([]rune(name)) >= minContainedName && strings.Contains(news, name):
			loose = append(loose, meet)
		}
	}
	candidates := exact
	if len(candidates) == 0 {
		candidates = loose
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	// 日付と会場は、名前で区別できなかった大会を分けるときだけ使う。
	var narrowed []KnownMeet
	for _, meet := range candidates {
		if agreesOnDateOrVenue(item, meet) {
			narrowed = append(narrowed, meet)
		}
	}
	if len(narrowed) == 1 {
		return narrowed[0], true
	}
	return KnownMeet{}, false
}

func agreesOnDateOrVenue(item NewsItem, meet KnownMeet) bool {
	if day, ok := parseDay(item.EventDate); ok {
		end := meet.StartsOn
		if meet.EndsOn != nil {
			end = *meet.EndsOn
		}
		if !day.Before(meet.StartsOn) && !day.After(end) {
			return true
		}
	}
	venue := normalizeVenue(item.Venue)
	place := normalizeVenue(meet.Location)
	return venue != "" && place != "" && (strings.Contains(venue, place) || strings.Contains(place, venue))
}

func normalizeVenue(s string) string {
	return strings.Join(strings.Fields(strings.NewReplacer("　", " ").Replace(s)), "")
}

// meetNameNoise は、ニュースのタイトルが大会名の前後に付け足す言葉。
var meetNameNoise = strings.NewReplacer(
	"掲載のお知らせ", "", "のお知らせ", "", "掲載", "", "要項", "", "【", "", "】", "", "「", "", "」", "",
	"　", "", " ", "", "\t", "",
)

// normalizeMeetName makes 第七十一回県中学陸上 要項掲載 and 第71回県中学陸上 comparable.
func normalizeMeetName(s string) string {
	return kanjiNumbersToDigits(normalizeDigits(meetNameNoise.Replace(s)))
}

func kanjiNumbersToDigits(s string) string {
	var out strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); {
		if _, ok := kanjiDigit(runes[i]); !ok {
			out.WriteRune(runes[i])
			i++
			continue
		}
		j := i
		for j < len(runes) {
			if _, ok := kanjiDigit(runes[j]); !ok {
				break
			}
			j++
		}
		run := runes[i:j]
		// A lone 十 or 百 inside a word (十種競技) is part of the name in both places, so converting it is harmless.
		out.WriteString(fmt.Sprint(kanjiRunValue(run)))
		i = j
	}
	return out.String()
}

func kanjiDigit(r rune) (int, bool) {
	switch r {
	case '〇', '零':
		return 0, true
	case '一':
		return 1, true
	case '二':
		return 2, true
	case '三':
		return 3, true
	case '四':
		return 4, true
	case '五':
		return 5, true
	case '六':
		return 6, true
	case '七':
		return 7, true
	case '八':
		return 8, true
	case '九':
		return 9, true
	case '十':
		return 10, true
	case '百':
		return 100, true
	}
	return 0, false
}

// kanjiRunValue reads 七十一 as 71 and 二〇二六 as 2026.
func kanjiRunValue(run []rune) int {
	positional := true
	for _, r := range run {
		if r == '十' || r == '百' {
			positional = false
		}
	}
	if positional {
		n := 0
		for _, r := range run {
			d, _ := kanjiDigit(r)
			n = n*10 + d
		}
		return n
	}
	total, current := 0, 0
	for _, r := range run {
		d, _ := kanjiDigit(r)
		switch d {
		case 10, 100:
			if current == 0 {
				current = 1
			}
			total += current * d
			current = 0
		default:
			current = current*10 + d
		}
	}
	return total + current
}

// notGuidelines は、要項の隣に並んでいて、要項と取り違えてはいけない文書。
var notGuidelines = []string{"申込", "申し込み", "結果", "一覧", "タイムテーブル", "競技日程", "エントリー"}

func isNotGuidelinesText(s string) bool {
	for _, word := range notGuidelines {
		if strings.Contains(s, word) {
			return true
		}
	}
	return false
}

// GuidelinesFromArticle finds the guidelines of a meet on its news article. A link whose text says 要項 is
// the guidelines. If 要項 is only written in the article text, the article itself is. Entry forms, results,
// lists, timetables and schedules are never guidelines.
func GuidelinesFromArticle(r io.Reader, articleURL string) (string, bool) {
	base, err := url.Parse(articleURL)
	if err != nil {
		return "", false
	}
	doc, err := html.Parse(r)
	if err != nil {
		return "", false
	}
	var links []LinkRef
	var text strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
			text.WriteByte(' ')
			return
		}
		if n.Type == html.ElementNode {
			if skipTextTag(n.Data) {
				return
			}
			if n.Data == "a" {
				for _, attr := range n.Attr {
					if attr.Key != "href" {
						continue
					}
					if abs, ok := absoluteURL(base, attr.Val); ok {
						links = append(links, LinkRef{URL: abs, Text: strings.Join(strings.Fields(cellText(n)), " ")})
					}
					break
				}
				// リンクの文字はそれだけで判断するので、記事の文字には数えない。
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	for _, link := range links {
		if strings.Contains(link.Text, "要項") && !isNotGuidelinesText(link.Text) {
			return link.URL, true
		}
	}
	// A button image has no link text, so the file's own path (taikaiyoukou, yoko…) is what says it is the guidelines.
	for _, link := range links {
		if link.Text == "" && pathHasGuidelines(link.URL) {
			return link.URL, true
		}
	}
	if strings.Contains(text.String(), "要項") {
		return articleURL, true
	}
	return "", false
}

// FindArticleGuidelines は、ニュース記事を1回開き、そこに載っている要項の URL を返す。
func FindArticleGuidelines(ctx context.Context, item NewsItem) (string, bool, error) {
	res, finalURL, err := get(ctx, item.ArticleURL)
	if err != nil {
		return "", false, err
	}
	defer res.Body.Close()
	if isPDFResponse(res, finalURL) {
		// 記事そのものが文書のとき、タイトルが要項と言っている場合だけ要項とみなす。
		return finalURL, strings.Contains(item.Title, "要項") && !isNotGuidelinesText(item.Title), nil
	}
	reader, err := charset.NewReader(res.Body, res.Header.Get("Content-Type"))
	if err != nil {
		return "", false, err
	}
	link, ok := GuidelinesFromArticle(io.LimitReader(reader, 8<<20), finalURL)
	return link, ok, nil
}
