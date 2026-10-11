package competitions

import (
	"sort"
	"time"
)

const DailyLimit = 7

// RefreshAfter is how long a successful import stays fresh.
const RefreshAfter = 7 * 24 * time.Hour

// DueSources returns up to DailyLimit sources that have never been imported
// or whose latest row is at least RefreshAfter old. Never-imported sources come first.
func DueSources(sources []Source, updated map[string]time.Time, now time.Time) []Source {
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
	for _, source := range sorted {
		imported, ok := freshTime(updated, source.Prefecture)
		if ok && now.Sub(imported) < RefreshAfter {
			continue
		}
		due = append(due, source)
		if len(due) == DailyLimit {
			break
		}
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
