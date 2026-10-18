package competitions

import (
	"sort"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/jst"
)

// DailyRequestLimit は、1日1回の実行で使ってよい Gemini リクエスト数の既定値。
// 無料枠の1日あたりの上限はプロジェクトごとで、AI Studio にしか出ないので、報告されている中でいちばん低い値（モデルごとに1日20回）より十分少なくする。
const DailyRequestLimit = 15

// ImportWindowStart は、年間の日程表を取り込む今の期間が始まった時刻。now より後ではない、いちばん新しい4月1日か10月1日（日本時間）。
// 県は春にその年の日程表を出し、秋に直す県もあるので、県ごとに期間ごとに1回だけ取り込み、あとはそのままにする。
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

// InternationalRefreshAfter が短いのは、国際大会は Gemini を使わずに読むから。
const InternationalRefreshAfter = 7 * 24 * time.Hour

// Requests は、s の取り込みに使う Gemini のリクエスト数。リンクされた PDF はまとめて1回で送る。
func (s Source) Requests() int {
	if len(s.PDFLinks) > 0 {
		return 1 + len(s.ExtraURLs)
	}
	return len(s.URLs())
}

// DueSources は、今の期間が始まってからまだ取り込んでいないソースを、古い順に、Gemini のリクエストが limit に収まるだけ返す。
// 収まらないソースは後日に回し、その日にはいちばん先に並ぶ。
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
