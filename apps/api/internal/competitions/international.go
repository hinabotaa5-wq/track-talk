package competitions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/jst"
	"golang.org/x/net/html"
)

// InternationalPrefecture は、日本の県に結び付かない大会をしまう先。
const InternationalPrefecture = "00"

const (
	worldAthleticsCalendar = "https://worldathletics.org/competition/calendar-results"
	diamondLeagueCalendar  = "https://www.diamondleague.com/calendar/"
)

// SeasonYear は、now を含む陸上のシーズン。シーズンは4月に始まる。
func SeasonYear(now time.Time) int {
	t := now.In(jst.Zone)
	if t.Month() >= time.April {
		return t.Year()
	}
	return t.Year() - 1
}

// InternationalWindow は、今のシーズンと次のシーズン。
func InternationalWindow(now time.Time) (from, to time.Time, seasons []int) {
	year := SeasonYear(now)
	from = time.Date(year, time.April, 1, 0, 0, 0, 0, time.UTC)
	to = time.Date(year+2, time.March, 31, 0, 0, 0, 0, time.UTC)
	return from, to, []int{year, year + 1}
}

// FetchInternational は、World Athletics と Diamond League の日程を読む。
func FetchInternational(ctx context.Context, now time.Time) ([]Meet, error) {
	from, to, _ := InternationalWindow(now)
	events, err := fetchWorldAthletics(ctx, from, to)
	if err != nil {
		return nil, err
	}
	meetings, err := fetchDiamondLeague(ctx)
	if err != nil {
		return nil, err
	}
	return BuildInternationalMeets(events, meetings, from, to)
}

var worldAthleticsFilters = []url.Values{
	{"competitionGroupId": {"3806"}}, // World Athletics Series
	{"competitionGroupId": {"5"}},    // Olympic Games
	{"rankingCategoryId": {"4"}},     // OW
	{"rankingCategoryId": {"5"}},     // GW
	{"rankingCategoryId": {"11"}},    // DF
}

func fetchWorldAthletics(ctx context.Context, from, to time.Time) ([]waEvent, error) {
	seen := map[string]bool{}
	var events []waEvent
	for _, filter := range worldAthleticsFilters {
		page, err := fetchWorldAthleticsFilter(ctx, from, to, filter)
		if err != nil {
			return nil, err
		}
		for _, event := range page {
			key := event.key()
			if seen[key] {
				continue
			}
			seen[key] = true
			events = append(events, event)
		}
	}
	return events, nil
}

func fetchWorldAthleticsFilter(ctx context.Context, from, to time.Time, filter url.Values) ([]waEvent, error) {
	var all []waEvent
	seen := map[string]bool{}
	for offset := 0; offset < 300; offset += 100 {
		body, err := getBody(ctx, worldAthleticsPage(from, to, offset, filter))
		if err != nil {
			return nil, err
		}
		events, hits, err := parseWorldAthletics(body, from, to, filter)
		if err != nil {
			return nil, err
		}
		if hits > 300 {
			return nil, fmt.Errorf("世界陸連のカレンダーが多すぎます")
		}
		added := 0
		for _, event := range events {
			key := event.key()
			if seen[key] {
				continue
			}
			seen[key] = true
			all = append(all, event)
			added++
		}
		if added == 0 || len(all) >= hits {
			break
		}
	}
	return all, nil
}

func worldAthleticsPage(from, to time.Time, offset int, filter url.Values) string {
	query := url.Values{}
	query.Set("startDate", from.Format("2006-01-02"))
	query.Set("endDate", to.Format("2006-01-02"))
	query.Set("limit", "100")
	query.Set("offset", strconv.Itoa(offset))
	for key, values := range filter {
		for _, value := range values {
			query.Set(key, value)
		}
	}
	return worldAthleticsCalendar + "?" + query.Encode()
}

type waEvent struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	Venue            string  `json:"venue"`
	RankingCategory  string  `json:"rankingCategory"`
	CompetitionGroup string  `json:"competitionGroup"`
	StartDate        string  `json:"startDate"`
	EndDate          string  `json:"endDate"`
	WasURL           *string `json:"wasUrl"`
}

func (e waEvent) key() string {
	if e.ID != 0 {
		return strconv.Itoa(e.ID)
	}
	return e.Name + "|" + e.StartDate
}

func (e waEvent) pageURL() (string, string) {
	if e.WasURL == nil || strings.TrimSpace(*e.WasURL) == "" {
		return worldAthleticsCalendar, "世界陸連のカレンダー"
	}
	raw := strings.TrimSpace(*e.WasURL)
	if strings.HasPrefix(raw, "/") {
		return "https://worldathletics.org" + raw, "大会ページ"
	}
	return raw, "大会ページ"
}

type waParameters struct {
	StartDate          *string `json:"startDate"`
	EndDate            *string `json:"endDate"`
	RankingCategoryID  *int    `json:"rankingCategoryId"`
	CompetitionGroupID *int    `json:"competitionGroupId"`
}

var nextDataPattern = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)

func parseWorldAthletics(body []byte, from, to time.Time, filter url.Values) ([]waEvent, int, error) {
	match := nextDataPattern.FindSubmatch(body)
	if match == nil {
		return nil, 0, fmt.Errorf("世界陸連のカレンダーが見つかりません")
	}
	var parsed struct {
		Props struct {
			PageProps struct {
				InitialEvents struct {
					Hits       int          `json:"hits"`
					Results    []waEvent    `json:"results"`
					Parameters waParameters `json:"parameters"`
				} `json:"initialEvents"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(match[1], &parsed); err != nil {
		return nil, 0, fmt.Errorf("世界陸連のカレンダー: %w", err)
	}
	events := parsed.Props.PageProps.InitialEvents
	if err := parametersMatch(events.Parameters, from, to, filter); err != nil {
		return nil, 0, err
	}
	if events.Results == nil {
		return []waEvent{}, events.Hits, nil
	}
	return events.Results, events.Hits, nil
}

func parametersMatch(got waParameters, from, to time.Time, filter url.Values) error {
	if !dateParamMatches(got.StartDate, from) || !dateParamMatches(got.EndDate, to) {
		return fmt.Errorf("世界陸連のカレンダーの期間が違います")
	}
	if want := filter.Get("rankingCategoryId"); want != "" && !intParamMatches(got.RankingCategoryID, want) {
		return fmt.Errorf("世界陸連のカレンダーの区分が違います")
	}
	if want := filter.Get("competitionGroupId"); want != "" && !intParamMatches(got.CompetitionGroupID, want) {
		return fmt.Errorf("世界陸連のカレンダーの区分が違います")
	}
	return nil
}

func dateParamMatches(got *string, day time.Time) bool {
	if got == nil {
		return false
	}
	value := *got
	if len(value) >= 10 {
		value = value[:10]
	}
	return value == day.Format("2006-01-02")
}

func intParamMatches(got *int, want string) bool {
	if got == nil {
		return false
	}
	return strconv.Itoa(*got) == want
}

type diamondMeeting struct {
	StartsOn time.Time
	EndsOn   *time.Time
	Location string
	URL      string
}

var (
	calendarYearPattern = regexp.MustCompile(`Calendar (20\d{2})`)
	diamondDatePattern  = regexp.MustCompile(`^(\d{1,2})(?:-(\d{1,2}))? (January|February|March|April|May|June|July|August|September|October|November|December)$`)
	diamondPlacePattern = regexp.MustCompile(`\([A-Z]{3}\)$`)
	countryCodePattern  = regexp.MustCompile(`\s*\([A-Z]{3}\)$`)
)

var monthNames = map[string]time.Month{
	"January": time.January, "February": time.February, "March": time.March,
	"April": time.April, "May": time.May, "June": time.June,
	"July": time.July, "August": time.August, "September": time.September,
	"October": time.October, "November": time.November, "December": time.December,
}

func fetchDiamondLeague(ctx context.Context) ([]diamondMeeting, error) {
	body, err := getBody(ctx, diamondLeagueCalendar)
	if err != nil {
		return nil, err
	}
	return parseDiamondLeague(body, diamondLeagueCalendar)
}

func parseDiamondLeague(body []byte, pageURL string) ([]diamondMeeting, error) {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil, err
	}
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("ダイヤモンドリーグ: %w", err)
	}
	yearText := joinedText(doc)
	yearMatch := calendarYearPattern.FindStringSubmatch(yearText)
	if yearMatch == nil {
		return nil, fmt.Errorf("ダイヤモンドリーグの年度が見つかりません")
	}
	year, _ := strconv.Atoi(yearMatch[1])
	var meetings []diamondMeeting
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "group") && hasClass(n, "gap-4") {
			if meeting, ok := diamondCard(n, base, year); ok {
				meetings = append(meetings, meeting)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if len(meetings) == 0 {
		return nil, fmt.Errorf("ダイヤモンドリーグの日程が見つかりません")
	}
	return meetings, nil
}

func diamondCard(n *html.Node, base *url.URL, year int) (diamondMeeting, bool) {
	// 画面にリンクとして出すので、http(s) のものだけ受け取る。
	absolute, ok := absoluteURL(base, attr(n, "href"))
	if !ok {
		return diamondMeeting{}, false
	}
	var dateLine, place string
	for _, text := range textParts(n) {
		if diamondDatePattern.MatchString(text) {
			dateLine = text
		}
		if diamondPlacePattern.MatchString(text) {
			place = text
		}
	}
	if dateLine == "" || place == "" {
		return diamondMeeting{}, false
	}
	match := diamondDatePattern.FindStringSubmatch(dateLine)
	startDay, _ := strconv.Atoi(match[1])
	month := monthNames[match[3]]
	start := time.Date(year, month, startDay, 0, 0, 0, 0, time.UTC)
	if start.Month() != month || start.Day() != startDay {
		return diamondMeeting{}, false
	}
	var end *time.Time
	if match[2] != "" {
		endDay, _ := strconv.Atoi(match[2])
		parsed := time.Date(year, month, endDay, 0, 0, 0, 0, time.UTC)
		if parsed.Month() != month || parsed.Day() != endDay || !parsed.After(start) {
			return diamondMeeting{}, false
		}
		end = &parsed
	}
	return diamondMeeting{StartsOn: start, EndsOn: end, Location: place, URL: absolute}, true
}

// BuildInternationalMeets は、世界選手権、Diamond League、World Athletics の上位のパーミット区分の大会を残す。
func BuildInternationalMeets(events []waEvent, meetings []diamondMeeting, from, to time.Time) ([]Meet, error) {
	unused := map[string][]diamondMeeting{}
	for _, meeting := range meetings {
		if !inWindow(meeting.StartsOn, from, to) {
			continue
		}
		key := meeting.StartsOn.Format("2006-01-02")
		unused[key] = append(unused[key], meeting)
	}
	var meets []Meet
	seen := map[string]bool{}
	for _, event := range events {
		if !includeEvent(event) {
			continue
		}
		start, ok := parseDay(event.StartDate)
		if !ok || !inWindow(start, from, to) {
			continue
		}
		end := eventEnd(start, event.EndDate)
		location := cleanText(event.Venue)
		name := cleanText(event.Name)
		organizer, label := event.pageURL()
		source := worldAthleticsCalendar
		if isDiamondLeague(event) {
			if meeting, ok := takeDiamondMeeting(unused, start, location+" "+name); ok {
				organizer = meeting.URL
				label = "公式サイト"
				source = diamondLeagueCalendar
				name = diamondLeagueName(name, meeting.Location)
				if location == "" {
					location = meeting.Location
				}
				start, end = coverDates(start, end, meeting)
			}
		}
		meet := Meet{
			Name:         localizeMeetName(name),
			StartsOn:     start,
			EndsOn:       end,
			Location:     localizeLocation(location),
			Prefecture:   InternationalPrefecture,
			SeasonYear:   SeasonYear(start),
			OrganizerURL: organizer,
			SourceURL:    source,
			LinkLabel:    label,
		}
		identity := name + "|" + meet.StartsOn.Format("2006-01-02")
		if seen[identity] {
			continue
		}
		seen[identity] = true
		meets = append(meets, meet)
	}
	for _, leftover := range unused {
		for _, meeting := range leftover {
			city := strings.TrimSpace(countryCodePattern.ReplaceAllString(meeting.Location, ""))
			name := "Wanda Diamond League"
			if city != "" {
				name += " " + city
			}
			meet := Meet{
				Name:         localizeMeetName(name),
				StartsOn:     meeting.StartsOn,
				EndsOn:       meeting.EndsOn,
				Location:     localizeLocation(meeting.Location),
				Prefecture:   InternationalPrefecture,
				SeasonYear:   SeasonYear(meeting.StartsOn),
				OrganizerURL: meeting.URL,
				SourceURL:    diamondLeagueCalendar,
				LinkLabel:    "公式サイト",
			}
			identity := name + "|" + meet.StartsOn.Format("2006-01-02")
			if seen[identity] {
				continue
			}
			seen[identity] = true
			meets = append(meets, meet)
		}
	}
	if len(meets) == 0 {
		return nil, fmt.Errorf("国際大会がありません")
	}
	return meets, nil
}

func includeEvent(event waEvent) bool {
	if strings.TrimSpace(event.Name) == "" || strings.Contains(event.Name, "National Championships Window") {
		return false
	}
	switch event.RankingCategory {
	case "OW", "GW", "DF":
		return true
	}
	group := event.CompetitionGroup
	return strings.Contains(group, "World Athletics Series") ||
		strings.Contains(group, "Olympic Games") ||
		strings.Contains(group, "Wanda Diamond League")
}

func isDiamondLeague(event waEvent) bool {
	return strings.Contains(event.CompetitionGroup, "Diamond League")
}

// takeDiamondMeeting は、World Athletics の Diamond League の行を、大会のサイトの行と組み合わせる。
// 同じ大会が1日ずれて載っていることがあるので、都市が同じなら隣の日も同じとみなす。
func takeDiamondMeeting(unused map[string][]diamondMeeting, start time.Time, haystack string) (diamondMeeting, bool) {
	if meeting, ok := takeMeetingOn(unused, start, ""); ok {
		return meeting, true
	}
	for _, day := range []time.Time{start.AddDate(0, 0, -1), start.AddDate(0, 0, 1)} {
		if meeting, ok := takeMeetingOn(unused, day, haystack); ok {
			return meeting, true
		}
	}
	return diamondMeeting{}, false
}

func takeMeetingOn(unused map[string][]diamondMeeting, day time.Time, haystack string) (diamondMeeting, bool) {
	key := day.Format("2006-01-02")
	list := unused[key]
	for i, meeting := range list {
		if haystack != "" && !cityMatches(meeting.Location, haystack) {
			continue
		}
		unused[key] = append(list[:i], list[i+1:]...)
		return meeting, true
	}
	return diamondMeeting{}, false
}

func cityMatches(place, haystack string) bool {
	city := strings.TrimSpace(countryCodePattern.ReplaceAllString(place, ""))
	folded := foldCity(haystack)
	for _, token := range strings.FieldsFunc(city, func(r rune) bool {
		return r == '/' || r == ' ' || r == ',' || r == '-'
	}) {
		token = foldCity(token)
		if len([]rune(token)) < 4 {
			continue
		}
		if strings.Contains(folded, token) {
			return true
		}
	}
	return false
}

func foldCity(value string) string {
	value = strings.ToLower(value)
	return strings.NewReplacer(
		"ü", "u", "ö", "o", "ä", "a",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"á", "a", "à", "a", "â", "a", "ã", "a", "å", "a", "ą", "a", "æ", "a",
		"í", "i", "ì", "i", "î", "i", "ï", "i", "ī", "i",
		"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ø", "o", "œ", "o",
		"ú", "u", "ù", "u", "û", "u", "ū", "u",
		"ñ", "n", "ń", "n", "ç", "c", "č", "c",
		"š", "s", "ś", "s", "ş", "s", "ß", "s",
		"ž", "z", "ź", "z", "ż", "z",
		"ř", "r", "ğ", "g", "ł", "l", "đ", "d", "ð", "d", "ţ", "t", "ý", "y", "ÿ", "y", "ę", "e", "ě", "e", "ő", "o", "ű", "u",
	).Replace(value)
}

func coverDates(start time.Time, end *time.Time, meeting diamondMeeting) (time.Time, *time.Time) {
	earliest := start
	if meeting.StartsOn.Before(earliest) {
		earliest = meeting.StartsOn
	}
	latest := earliest
	if end != nil && end.After(latest) {
		latest = *end
	}
	if meeting.StartsOn.After(latest) {
		latest = meeting.StartsOn
	}
	if meeting.EndsOn != nil && meeting.EndsOn.After(latest) {
		latest = *meeting.EndsOn
	}
	if !latest.After(earliest) {
		return earliest, nil
	}
	copied := latest
	return earliest, &copied
}

func diamondLeagueName(name, place string) string {
	if name != "Wanda Diamond League" {
		return name
	}
	city := strings.TrimSpace(countryCodePattern.ReplaceAllString(place, ""))
	if city == "" {
		return name
	}
	return name + " " + city
}

func eventEnd(start time.Time, raw string) *time.Time {
	end, ok := parseDay(raw)
	if !ok || !end.After(start) {
		return nil
	}
	return &end
}

func inWindow(day, from, to time.Time) bool {
	return !day.Before(from) && !day.After(to)
}

// userAgent はWikimediaのUser-Agentポリシーに合わせ、連絡先を含める。連絡先のないUser-Agentは429で弾かれやすい。
const userAgent = "TrackTalk/1.0 (https://github.com/hinabotaa5-wq/track-talk)"

const maxGetAttempts = 4

// retryWaitCap は Retry-After が極端に長いときに待つ上限。
const retryWaitCap = 30 * time.Second

// retryBaseWait は Retry-After がないときの待ち時間の基準(試行ごとに倍にする)。
var retryBaseWait = 2 * time.Second

// getBodyClient は国際大会・国内主要大会・開催地の取得に使う。応答しないサイトが1つあるだけで
// 全体の時間を使い切らないよう、1回ごとに時間を区切る。
var getBodyClient = &http.Client{Timeout: 60 * time.Second}

// getBody は429と503のとき、Retry-Afterに従って待ってから最大 maxGetAttempts 回まで取得し直す。
func getBody(ctx context.Context, pageURL string) ([]byte, error) {
	for attempt := 1; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)
		res, err := getBodyClient.Do(req)
		if err != nil {
			return nil, err
		}
		if res.StatusCode == http.StatusOK {
			defer res.Body.Close()
			return io.ReadAll(io.LimitReader(res.Body, 8<<20))
		}
		res.Body.Close()
		retryable := res.StatusCode == http.StatusTooManyRequests || res.StatusCode == http.StatusServiceUnavailable
		if !retryable || attempt >= maxGetAttempts {
			return nil, fmt.Errorf("page %s: %s", pageURL, res.Status)
		}
		wait := retryBaseWait << (attempt - 1)
		if seconds, err := strconv.Atoi(strings.TrimSpace(res.Header.Get("Retry-After"))); err == nil && seconds >= 0 {
			wait = time.Duration(seconds) * time.Second
		}
		if wait > retryWaitCap {
			wait = retryWaitCap
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

func hasClass(n *html.Node, class string) bool {
	for _, part := range strings.Fields(attr(n, "class")) {
		if part == class {
			return true
		}
	}
	return false
}

func attr(n *html.Node, key string) string {
	for _, attribute := range n.Attr {
		if attribute.Key == key {
			return attribute.Val
		}
	}
	return ""
}

func textParts(n *html.Node) []string {
	var parts []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := strings.Join(strings.Fields(n.Data), " ")
			if text != "" {
				parts = append(parts, text)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return parts
}

func joinedText(n *html.Node) string {
	return strings.Join(textParts(n), " ")
}
