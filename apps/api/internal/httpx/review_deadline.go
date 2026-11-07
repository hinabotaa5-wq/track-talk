package httpx

import (
	"fmt"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/jst"
)

// 動画添削の期限（運営者決定 2026-11-02）。
//   - 返却期限：申込みから24時間。夜間も止まらない。
//   - 引受期限：コーチへの打診処理から90分（2026-11-02 運営者決定、23:59 再確認。起点はサーバ側の打診処理時刻で、Push の到達時刻ではない）。ただし日本時間の23時〜翌7時はカウントを止める。
//
// 夜間の判定は日本時間の壁時計で行い、サーバーのタイムゾーンには依存しない。
const (
	// 申込みから返却期限までの時間。
	reviewReturnWindow = 24 * time.Hour
	// コーチへの打診処理から引き受けるかどうか決めるまでの時間（夜間を除いて数える）。
	offerWindow = 90 * time.Minute
	// 引受タイマーを止める時間帯（日本時間）。quietStartHour 時ちょうどから止め、quietEndHour 時ちょうどから数え直す。
	quietStartHour = 23
	quietEndHour   = 7
)

// reviewDueAt は返却期限。申込みの時刻から24時間後。
func reviewDueAt(now time.Time) time.Time {
	return now.Add(reviewReturnWindow)
}

// offerExpiresAt は引受期限。start から window を、日本時間の23時〜翌7時を除いて数えた時刻。
// 例（90分）：02:00 → 08:30、22:45 → 08:15（23時までに15分、残り75分は7時から）、23:30 → 08:30。
func offerExpiresAt(start time.Time, window time.Duration) time.Time {
	t := start.In(jst.Zone)
	remaining := window
	for {
		day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, jst.Zone)
		quietStart := day.Add(quietStartHour * time.Hour)
		quietEnd := day.Add(quietEndHour * time.Hour)
		switch {
		case t.Before(quietEnd):
			// 0時〜7時は止まっている。7時から数え始める。
			t = quietEnd
		case !t.Before(quietStart):
			// 23時以降は止まっている。翌日の7時から数え始める。
			t = quietEnd.AddDate(0, 0, 1)
		default:
			if end := t.Add(remaining); !end.After(quietStart) {
				return end
			}
			remaining -= quietStart.Sub(t)
			t = quietStart
		}
	}
}

// offerDeadlineText は、通知に出す引受期限（日本時間）。例：「11月3日 7:30」。
func offerDeadlineText(t time.Time) string {
	t = t.In(jst.Zone)
	return fmt.Sprintf("%d月%d日 %d:%02d", t.Month(), t.Day(), t.Hour(), t.Minute())
}

// reminderWindow は、引受期限の何分前（カウント可能な時間）になったらリマインドを送るか（運営者決定 2026-11-08、本体 #420）。
const reminderWindow = 30 * time.Minute

// countableUntil は from から to までのうち、日本時間の23時〜翌7時を除いた時間。
// 引受期限までの残り（カウント可能な時間）を数える。to が from より前なら0以下になる。
func countableUntil(from, to time.Time) time.Duration {
	t := from.In(jst.Zone)
	end := to.In(jst.Zone)
	var total time.Duration
	for t.Before(end) {
		day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, jst.Zone)
		quietStart := day.Add(quietStartHour * time.Hour)
		quietEnd := day.Add(quietEndHour * time.Hour)
		switch {
		case t.Before(quietEnd):
			// 0時〜7時は止まっている。7時から数え直す。
			t = quietEnd
		case !t.Before(quietStart):
			// 23時以降は止まっている。翌日の7時から数え直す。
			t = quietEnd.AddDate(0, 0, 1)
		default:
			segEnd := end
			if quietStart.Before(segEnd) {
				segEnd = quietStart
			}
			total += segEnd.Sub(t)
			t = segEnd
		}
	}
	return total
}

// reminderDue は、打診のリマインドを送る時刻か。引受期限までのカウント可能な残りが
// reminderWindow 以下で、まだ期限を過ぎていないときだけ真。夜間停止を跨ぐ打診も残り時間で判定する。
func reminderDue(now, expiresAt time.Time) bool {
	remaining := countableUntil(now, expiresAt)
	return remaining > 0 && remaining <= reminderWindow
}
