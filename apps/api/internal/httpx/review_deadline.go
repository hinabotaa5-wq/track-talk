package httpx

import (
	"fmt"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/jst"
)

// 添削の返却期限は依頼が対応中一覧に表示されてから24時間。
const reviewReturnWindow = 24 * time.Hour

func reviewDueAt(now time.Time) time.Time {
	return now.Add(reviewReturnWindow)
}

func reviewDeadlineText(t time.Time) string {
	t = t.In(jst.Zone)
	return fmt.Sprintf("%d月%d日 %d:%02d", t.Month(), t.Day(), t.Hour(), t.Minute())
}
