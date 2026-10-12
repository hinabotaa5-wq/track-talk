package competitions

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

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

// Draft is one meet as returned by Gemini, before the URLs are checked.
type Draft struct {
	Name          string `json:"name"`
	StartsOn      string `json:"starts_on"`
	EndsOn        string `json:"ends_on"`
	Location      string `json:"location"`
	GuidelinesURL string `json:"guidelines_url"`
	DistrictURL   string `json:"district_url"`
}

func BuildMeets(page Page, drafts []Draft, prefecture, sourceURL string) ([]Meet, error) {
	var meets []Meet
	seen := map[string]bool{}
	for _, draft := range drafts {
		meet, ok := acceptDraft(page, draft, prefecture, sourceURL)
		if !ok {
			continue
		}
		key := meet.Name + "|" + meet.StartsOn.Format("2006-01-02")
		if seen[key] {
			continue
		}
		seen[key] = true
		meets = append(meets, meet)
	}
	if len(meets) == 0 {
		return nil, fmt.Errorf("大会がありません")
	}
	return meets, nil
}

var embeddedURL = regexp.MustCompile(`https?://\S+`)

func isCompetitionName(name string) bool {
	for _, word := range []string{"理事会", "講習会", "クリニック", "練習会", "【中止】", "（中止）", "[中止]"} {
		if strings.Contains(name, word) {
			return false
		}
	}
	return true
}

func cleanText(raw string) string {
	return strings.Join(strings.Fields(embeddedURL.ReplaceAllString(raw, " ")), " ")
}

// editionPrefix is a leading edition number such as （６８）or (R8) that some schedules put before the meet name.
var editionPrefix = regexp.MustCompile(`^[（(]\s*[ＲR]?\s*[0-9０-９]+\s*[）)]\s*`)

func cleanName(raw string) string {
	name := cleanText(raw)
	if rest := editionPrefix.ReplaceAllString(name, ""); rest != "" {
		return rest
	}
	return name
}

var spaceBetweenWide = regexp.MustCompile(`([\p{Han}\p{Hiragana}\p{Katakana}])[ \t\x{3000}]+([\p{Han}\p{Hiragana}\p{Katakana}])`)

// cleanLocation joins place names that a schedule spaces out to fill a narrow cell (倉 敷 -> 倉敷).
func cleanLocation(raw string) string {
	text := cleanText(raw)
	for {
		next := spaceBetweenWide.ReplaceAllString(text, "$1$2")
		if next == text {
			return text
		}
		text = next
	}
}

func acceptDraft(page Page, draft Draft, prefecture, sourceURL string) (Meet, bool) {
	name := cleanName(draft.Name)
	if name == "" || !isCompetitionName(name) {
		return Meet{}, false
	}
	start, ok := parseDay(draft.StartsOn)
	if !ok || !inSeason(start, page.SeasonYear) {
		return Meet{}, false
	}
	var end *time.Time
	if draft.EndsOn != "" {
		parsed, ok := parseDay(draft.EndsOn)
		if !ok || parsed.Before(start) || !inSeason(parsed, page.SeasonYear) {
			return Meet{}, false
		}
		if !parsed.Equal(start) {
			end = &parsed
		}
	}
	organizer, label := destination(page, draft.GuidelinesURL, draft.DistrictURL, sourceURL)
	if organizer == sourceURL && label == "" {
		if rowURL, rowLabel, ok := page.destinationForName(name); ok {
			organizer, label = rowURL, rowLabel
		}
	}
	return Meet{
		Name:         name,
		StartsOn:     start,
		EndsOn:       end,
		Location:     cleanLocation(draft.Location),
		Prefecture:   prefecture,
		SeasonYear:   page.SeasonYear,
		OrganizerURL: organizer,
		SourceURL:    sourceURL,
		LinkLabel:    label,
	}, true
}

func destination(page Page, guidelinesURL, districtURL, sourceURL string) (string, string) {
	if link, ok := findLink(page, guidelinesURL, isGuidelines); ok {
		return link.URL, "要項"
	}
	if link, ok := findLink(page, districtURL, isGuidelines); ok {
		return link.URL, "要項"
	}
	if link, ok := findLink(page, districtURL, isDistrict); ok {
		return link.URL, districtLabel(link.Text)
	}
	if link, ok := findLink(page, guidelinesURL, isDistrict); ok {
		return link.URL, districtLabel(link.Text)
	}
	return sourceURL, ""
}

func (page Page) destinationForName(name string) (string, string, bool) {
	want := cleanText(name)
	if want == "" {
		return "", "", false
	}
	var partial row
	var partialLinks []LinkRef
	for _, row := range page.rows {
		rowName := cleanText(nameCell(row))
		links := guidelinesColumnLinks(row)
		if rowName == "" || len(links) == 0 {
			continue
		}
		if rowName == want {
			return chooseColumnLink(row, links)
		}
		if strings.Contains(rowName, want) || strings.Contains(want, rowName) {
			partial = row
			partialLinks = links
		}
	}
	if len(partialLinks) == 0 {
		return "", "", false
	}
	return chooseColumnLink(partial, partialLinks)
}

func nameCell(row row) string {
	for _, cell := range row.Cells {
		header := cell.Header
		if strings.Contains(header, "要項") {
			continue
		}
		if strings.Contains(header, "競技会") || strings.Contains(header, "行事") || strings.Contains(header, "大会") || strings.Contains(header, "主催") {
			return cell.Text
		}
	}
	if len(row.Cells) > 1 {
		return row.Cells[1].Text
	}
	return ""
}

func guidelinesColumnLinks(row row) []LinkRef {
	var links []LinkRef
	for _, cell := range row.Cells {
		if strings.Contains(cell.Header, "要項") {
			links = append(links, cell.Links...)
		}
	}
	return links
}

func chooseColumnLink(row row, links []LinkRef) (string, string, bool) {
	for _, link := range links {
		if isGuidelines(link) {
			return link.URL, "要項", true
		}
	}
	for _, link := range links {
		if isDistrict(link) {
			return link.URL, districtLabel(link.Text), true
		}
	}
	if len(links) == 0 {
		return "", "", false
	}
	return links[0].URL, organizerLabel(row), true
}

func organizerLabel(row row) string {
	for _, cell := range row.Cells {
		header := cell.Header
		if strings.Contains(header, "要項") || strings.Contains(header, "競技会") || strings.Contains(header, "行事") || strings.Contains(header, "大会") {
			continue
		}
		text := strings.TrimSpace(cell.Text)
		if isDistrictText(text) && len([]rune(text)) <= 24 {
			return text
		}
	}
	return "ホームページ"
}

func findLink(page Page, raw string, match func(LinkRef) bool) (LinkRef, bool) {
	if strings.TrimSpace(raw) == "" {
		return LinkRef{}, false
	}
	for _, link := range page.Links {
		if sameURL(link.URL, raw) && match(link) {
			return link, true
		}
	}
	return LinkRef{}, false
}

func isGuidelines(link LinkRef) bool {
	if strings.Contains(link.Text, "要項") || pathHasGuidelines(link.URL) {
		return true
	}
	if strings.Contains(link.Header, "要項") && !isDistrictText(link.Text) && (link.Text == "" || isDocument(link.URL)) {
		return true
	}
	return false
}

func isDistrict(link LinkRef) bool {
	if isGuidelines(link) {
		return false
	}
	return isDistrictText(link.Text)
}

func isDistrictText(text string) bool {
	return strings.Contains(text, "ホームページ") ||
		strings.Contains(text, "陸協") ||
		strings.Contains(text, "競技協会") ||
		strings.Contains(text, "スポ協") ||
		strings.Contains(text, "体連") ||
		strings.Contains(text, "公式")
}

func districtLabel(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return "ホームページ"
	}
	return text
}

func pathHasGuidelines(raw string) bool {
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

func sameURL(a, b string) bool {
	return strings.TrimRight(strings.TrimSpace(a), "/") == strings.TrimRight(strings.TrimSpace(b), "/")
}

func parseDay(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02", "2006-1-2", "2006/01/02", "2006/1/2"} {
		parsed, err := time.Parse(layout, raw)
		if err != nil {
			continue
		}
		return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC), true
	}
	return time.Time{}, false
}

func inSeason(day time.Time, season int) bool {
	start := time.Date(season, time.April, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(season+1, time.April, 1, 0, 0, 0, 0, time.UTC)
	return !day.Before(start) && day.Before(end)
}
