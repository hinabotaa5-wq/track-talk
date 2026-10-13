package competitions

import (
	"sort"
	"time"
)

// DailyRequestLimit is the default number of Gemini requests one daily run may make.
// The free tier's requests-per-day limit is per project and shown only in AI Studio,
// so this stays well under the lowest reported figure (20 per model per day).
const DailyRequestLimit = 15

// RefreshAfter is how long a successful prefecture import stays fresh.
const RefreshAfter = 10 * 24 * time.Hour

// InternationalRefreshAfter is shorter because international meets are read without Gemini.
const InternationalRefreshAfter = 7 * 24 * time.Hour

// Requests is how many Gemini requests importing s takes. Linked PDFs are sent together in one request.
func (s Source) Requests() int {
	if len(s.PDFLinks) > 0 {
		return 1 + len(s.ExtraURLs)
	}
	return len(s.URLs())
}

// DueSources returns sources that have never been imported or whose latest row is at least
// RefreshAfter old, oldest first, as long as their Gemini requests fit in limit.
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
	var due []Source
	remaining := limit
	for _, source := range sorted {
		imported, ok := freshTime(updated, source.Prefecture)
		if ok && now.Sub(imported) < RefreshAfter {
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
