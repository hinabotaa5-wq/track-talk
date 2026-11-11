package competitions

import (
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

var embeddedURL = regexp.MustCompile(`https?://\S+`)

func cleanText(raw string) string {
	return strings.Join(strings.Fields(embeddedURL.ReplaceAllString(raw, " ")), " ")
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
