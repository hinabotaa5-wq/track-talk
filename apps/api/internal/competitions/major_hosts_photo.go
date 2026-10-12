package competitions

import (
	"context"
	"encoding/json"
	"net/url"
	"path"
	"regexp"
	"strings"
)

const (
	commonsAPI  = "https://commons.wikimedia.org/w/api.php"
	englishWiki = "https://en.wikipedia.org/w/api.php"
	// 960 is a Wikimedia thumbnail step. A 1200px request is served as a 1280px file.
	thumbWidth = "960"
)

// cityLandscape is the skyline editors put first on the English Wikipedia city article.
// Those files live on Wikimedia Commons. A skyline category is the fallback.
func cityLandscape(ctx context.Context, host MajorHost) string {
	english := englishCityName(ctx, host)
	if english == "" {
		return ""
	}
	if image := englishSkyline(ctx, english); image != "" {
		return image
	}
	return commonsSkyline(ctx, english)
}

func englishCityName(ctx context.Context, host MajorHost) string {
	title := strings.TrimSpace(host.City)
	if page, ok := wikiTitle(host.CityPageURL); ok {
		title = page
	}
	if title == "" {
		return ""
	}
	query := url.Values{}
	query.Set("action", "query")
	query.Set("format", "json")
	query.Set("formatversion", "2")
	query.Set("prop", "langlinks")
	query.Set("lllang", "en")
	query.Set("redirects", "1")
	query.Set("titles", title)
	body, err := getBody(ctx, wikiAPI+"?"+query.Encode())
	if err != nil {
		return ""
	}
	return englishLangTitle(body)
}

func englishLangTitle(body []byte) string {
	var parsed struct {
		Query struct {
			Pages []struct {
				LangLinks []struct {
					Title string `json:"title"`
				} `json:"langlinks"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	for _, page := range parsed.Query.Pages {
		if len(page.LangLinks) == 0 {
			continue
		}
		title := strings.TrimSpace(page.LangLinks[0].Title)
		if cut := strings.Index(title, " ("); cut > 0 {
			title = strings.TrimSpace(title[:cut])
		}
		return title
	}
	return ""
}

func englishSkyline(ctx context.Context, english string) string {
	query := url.Values{}
	query.Set("action", "parse")
	query.Set("page", english)
	query.Set("prop", "wikitext")
	query.Set("section", "0")
	query.Set("redirects", "1")
	query.Set("format", "json")
	query.Set("formatversion", "2")
	body, err := getBody(ctx, englishWiki+"?"+query.Encode())
	if err != nil {
		return ""
	}
	file, ok := leadCityFile(leadWikitext(body))
	if !ok {
		return ""
	}
	return commonsThumbnail(ctx, file)
}

func leadWikitext(body []byte) string {
	var parsed struct {
		Parse struct {
			Wikitext json.RawMessage `json:"wikitext"`
		} `json:"parse"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	raw := strings.TrimSpace(string(parsed.Parse.Wikitext))
	if raw == "" {
		return ""
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(parsed.Parse.Wikitext, &text); err != nil {
			return ""
		}
		return text
	}
	var boxed struct {
		Star string `json:"*"`
	}
	if err := json.Unmarshal(parsed.Parse.Wikitext, &boxed); err != nil {
		return ""
	}
	return boxed.Star
}

func leadCityFile(wikitext string) (string, bool) {
	for _, name := range []string{"image_skyline", "image1", "image"} {
		file, ok := photoFileName(wikiParam(wikitext, name))
		if ok {
			return file, true
		}
	}
	return "", false
}

func wikiParam(text, name string) string {
	pattern := regexp.MustCompile(`(?m)^[ \t]*\|[ \t]*` + regexp.QuoteMeta(name) + `[ \t]*=[ \t]*(.*)$`)
	match := pattern.FindStringSubmatch(text)
	if match == nil {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func photoFileName(raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	name = strings.TrimPrefix(name, "[[")
	if cut := strings.Index(name, "]]"); cut >= 0 {
		name = name[:cut]
	}
	if cut := strings.Index(name, "|"); cut >= 0 {
		name = name[:cut]
	}
	name = strings.TrimSpace(name)
	lower := strings.ToLower(name)
	for _, prefix := range []string{"file:", "image:"} {
		if strings.HasPrefix(lower, prefix) {
			name = strings.TrimSpace(name[len(prefix):])
			lower = strings.ToLower(name)
			break
		}
	}
	if name == "" || strings.Contains(name, "{{") || strings.Contains(name, "}}") {
		return "", false
	}
	if !strings.HasSuffix(lower, ".jpg") && !strings.HasSuffix(lower, ".jpeg") && !strings.HasSuffix(lower, ".png") && !strings.HasSuffix(lower, ".webp") {
		return "", false
	}
	if rejectedPhotoName(name) {
		return "", false
	}
	return name, true
}

func rejectedPhotoName(name string) bool {
	bad := map[string]bool{
		"montage": true, "collage": true, "logo": true, "flag": true,
		"map": true, "maps": true, "locator": true, "emblem": true,
		"seal": true, "diagram": true, "icon": true, "icons": true,
		"symbol": true, "symbols": true, "svg": true, "banner": true,
		"pictogram": true, "coat": true,
	}
	for _, token := range photoTokens(name) {
		if bad[token] {
			return true
		}
	}
	return false
}

func photoTokens(name string) []string {
	var tokens []string
	var current []rune
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' {
			current = append(current, r)
			continue
		}
		if len(current) > 0 {
			tokens = append(tokens, string(current))
			current = nil
		}
	}
	if len(current) > 0 {
		tokens = append(tokens, string(current))
	}
	return tokens
}

func commonsSkyline(ctx context.Context, english string) string {
	for _, category := range []string{
		"Category:Skylines of " + english,
		"Category:Views of " + english,
		"Category:Panoramics of " + english,
		"Category:Skylines in " + english,
		"Category:Views in " + english,
	} {
		if image := chooseCityPhoto(commonsCategory(ctx, category)); image != "" {
			return image
		}
	}
	return chooseCityPhoto(commonsSearch(ctx, `"`+english+`" skyline`))
}

func commonsCategory(ctx context.Context, category string) []commonsImage {
	return commonsImages(ctx, url.Values{
		"generator":  {"categorymembers"},
		"gcmtitle":   {category},
		"gcmtype":    {"file"},
		"gcmlimit":   {"40"},
		"prop":       {"imageinfo"},
		"iiprop":     {"url|size|mime"},
		"iiurlwidth": {thumbWidth},
	})
}

func commonsSearch(ctx context.Context, search string) []commonsImage {
	return commonsImages(ctx, url.Values{
		"generator":    {"search"},
		"gsrsearch":    {search},
		"gsrnamespace": {"6"},
		"gsrlimit":     {"20"},
		"prop":         {"imageinfo"},
		"iiprop":       {"url|size|mime"},
		"iiurlwidth":   {thumbWidth},
	})
}

func commonsThumbnail(ctx context.Context, fileName string) string {
	images := commonsImages(ctx, url.Values{
		"titles":     {"File:" + fileName},
		"prop":       {"imageinfo"},
		"iiprop":     {"url|size|mime"},
		"iiurlwidth": {thumbWidth},
	})
	if len(images) == 0 || !photoMIME(images[0].MIME) || rejectedPhotoName(images[0].Title) {
		return ""
	}
	return images[0].URL
}

type commonsImage struct {
	Title  string
	URL    string
	Width  int
	Height int
	MIME   string
}

func commonsImages(ctx context.Context, query url.Values) []commonsImage {
	query.Set("action", "query")
	query.Set("format", "json")
	query.Set("formatversion", "2")
	body, err := getBody(ctx, commonsAPI+"?"+query.Encode())
	if err != nil {
		return nil
	}
	return parseCommonsImages(body)
}

func parseCommonsImages(body []byte) []commonsImage {
	var parsed struct {
		Query struct {
			Pages []struct {
				Title     string `json:"title"`
				Missing   bool   `json:"missing"`
				ImageInfo []struct {
					ThumbURL string `json:"thumburl"`
					URL      string `json:"url"`
					Width    int    `json:"width"`
					Height   int    `json:"height"`
					MIME     string `json:"mime"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil
	}
	var images []commonsImage
	for _, page := range parsed.Query.Pages {
		if page.Missing || len(page.ImageInfo) == 0 {
			continue
		}
		info := page.ImageInfo[0]
		photoURL := info.ThumbURL
		if photoURL == "" {
			photoURL = info.URL
		}
		images = append(images, commonsImage{
			Title: page.Title, URL: canonicalPhotoURL(photoURL), Width: info.Width, Height: info.Height, MIME: info.MIME,
		})
	}
	return images
}

func chooseCityPhoto(images []commonsImage) string {
	bestScore := -1
	bestURL := ""
	for _, image := range images {
		score := cityPhotoScore(image)
		if score > bestScore {
			bestScore = score
			bestURL = image.URL
		}
	}
	if bestScore < 0 {
		return ""
	}
	return bestURL
}

func cityPhotoScore(image commonsImage) int {
	if image.URL == "" || image.Width < 800 || image.Height <= 0 || image.Width <= image.Height {
		return -1
	}
	if !photoMIME(image.MIME) || rejectedPhotoName(image.Title) {
		return -1
	}
	ratio := float64(image.Width) / float64(image.Height)
	if ratio > 2.4 {
		return -1
	}
	delta := ratio - 4.0/3.0
	if delta < 0 {
		delta = -delta
	}
	score := image.Width + int((1.2-delta)*10000)
	for _, token := range photoTokens(image.Title) {
		if token == "skyline" {
			score += 8000
			break
		}
	}
	return score
}

func canonicalPhotoURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return strings.TrimSpace(raw)
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func photoMIME(mime string) bool {
	switch strings.ToLower(mime) {
	case "image/jpeg", "image/jpg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func wikiCityLead(ctx context.Context, host MajorHost) string {
	for _, page := range photoPages(host) {
		image, err := wikiThumbnail(ctx, page)
		if err != nil || !usableLeadPhoto(image) {
			continue
		}
		return image
	}
	if host.City == "" {
		return ""
	}
	image, err := wikiSummaryThumbnail(ctx, host.City)
	if err != nil || !usableLeadPhoto(image) {
		return ""
	}
	return image
}

func usableLeadPhoto(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Path == "" {
		return false
	}
	name, err := url.PathUnescape(path.Base(parsed.Path))
	if err != nil {
		name = path.Base(parsed.Path)
	}
	_, ok := photoFileName(name)
	return ok
}
