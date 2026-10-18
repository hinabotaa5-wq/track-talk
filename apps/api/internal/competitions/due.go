package competitions

import (
	"sort"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/jst"
)

// DailyRequestLimit is the default number of Gemini requests one daily run may make.
// The free tier's requests-per-day limit is per project and shown only in AI Studio,
// so this stays well under the lowest reported figure (20 per model per day).
const DailyRequestLimit = 15

// ImportWindowStart is when the current annual-schedule import window opened: the latest April 1 or
// October 1 (Japan time) not after now. Prefectures publish the year's table in spring and some revise it
// in autumn, so each prefecture is imported once per window and then left alone.
func ImportWindowStart(now time.Time) time.Time {
	local := now.In(jst.Zone)
	year := local.Year()
	for _, month := range []time.Month{time.October, time.April} {
		if start := time.Date(year, month, 1, 0, 0, 0, 0, jst.Zone); !start.After(local) {
			return start
		}
	}
	return time.Date(year-1, time.October, 1, 0, 0, 0, 0, jst.Zone)
}

// InternationalRefreshAfter is shorter because international meets are read without Gemini.
const InternationalRefreshAfter = 7 * 24 * time.Hour

// Requests is how many Gemini requests importing s takes. Linked PDFs are sent together in one request.
func (s Source) Requests() int {
	if len(s.PDFLinks) > 0 {
		return 1 + len(s.ExtraURLs)
	}
	return len(s.URLs())
}

// DueSources returns sources that have not been imported since the current window opened, oldest first, as long as their Gemini requests fit in limit.
// A source that does not fit is left for a later day, when it will be first in line.
func DueSources(sources []Source, updated map[string]time.Time, now time.Time, limit int) []Source {
	sorted := append([]Source(nil), sources...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, leftOK := freshTime(updated, sorted[i].Prefecture)
		right, rightOK := freshTime(updated, sorted[j].Prefecture)
		if leftOK != rightOK {
			return !leftOK
		}
		if !leftOK {
			return sorted[i].Prefecture < sorted[j].Prefecture
		}
		if !left.Equal(right) {
			return left.Before(right)
		}
		return sorted[i].Prefecture < sorted[j].Prefecture
	})
	windowStart := ImportWindowStart(now)
	var due []Source
	remaining := limit
	for _, source := range sorted {
		imported, ok := freshTime(updated, source.Prefecture)
		if ok && !imported.Before(windowStart) {
			continue
		}
		cost := source.Requests()
		if cost == 0 || cost > remaining {
			continue
		}
		due = append(due, source)
		remaining -= cost
	}
	return due
}

func freshTime(updated map[string]time.Time, prefecture string) (time.Time, bool) {
	imported, ok := updated[prefecture]
	if !ok || imported.IsZero() {
		return time.Time{}, false
	}
	return imported, true
}
