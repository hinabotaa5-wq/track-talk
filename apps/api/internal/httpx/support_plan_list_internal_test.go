package httpx

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

// 公平なローテーション：同じ日は同じ順番、日が変わると先頭が入れ替わり、どのコーチも先頭に来る。
func TestRotateCoachesIsDailyAndFair(t *testing.T) {
	ids := make([]uuid.UUID, 5)
	for i := range ids {
		ids[i] = uuid.New()
	}
	day := time.Date(2026, 12, 1, 9, 0, 0, 0, jst.Zone)
	a, b := rotateCoaches(ids, day), rotateCoaches(ids, day.Add(10*time.Hour))
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("order changed within the same day")
		}
	}
	first := map[uuid.UUID]int{}
	for d := 0; d < 400; d++ {
		first[rotateCoaches(ids, day.AddDate(0, 0, d))[0]]++
	}
	for _, id := range ids {
		if first[id] < 40 { // 400日で平均80回。極端な偏りがないこと
			t.Fatalf("coach rarely first: %v", first)
		}
	}
}
