package httpx

import (
	"testing"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

func TestBuildInjuryStats(t *testing.T) {
	parts := []db.CountInjuryPostsByPartRow{
		{BodyPart: "jumpers-knee", Posts: 7},
		{BodyPart: "sprain", Posts: 2},
	}
	stages := []db.InjuryStageStatsRow{
		{BodyPart: "jumpers-knee", Stage: "jogging", Reached: 5, MedianDays: 41.6},
		{BodyPart: "jumpers-knee", Stage: "rehab", Reached: 6, MedianDays: 12.4},
		{BodyPart: "jumpers-knee", Stage: "race", Reached: 4, MedianDays: 90},
		{BodyPart: "sprain", Stage: "rehab", Reached: 1, MedianDays: 3},
	}
	got := buildInjuryStats(parts, stages)

	if got.MinSample != 5 || len(got.Parts) != 2 {
		t.Fatalf("unexpected shape %+v", got)
	}
	// 部位は一覧の定義順(足首 → 膝)で返る。
	if got.Parts[0].BodyPart != "sprain" || got.Parts[1].BodyPart != "jumpers-knee" {
		t.Fatalf("part order %+v", got.Parts)
	}
	ankle := got.Parts[0]
	if ankle.Posts != 2 || len(ankle.Stages) != 1 || ankle.Stages[0].MedianDays != nil {
		t.Fatalf("ankle below sample must hide the median: %+v", ankle)
	}
	knee := got.Parts[1]
	if len(knee.Stages) != 3 {
		t.Fatalf("knee stages %+v", knee.Stages)
	}
	// 段階は rehab → jogging → race の順で、中央値は四捨五入した日数。下限未満(race)は nil。
	if knee.Stages[0].Stage != "rehab" || knee.Stages[0].MedianDays == nil || *knee.Stages[0].MedianDays != 12 {
		t.Fatalf("rehab %+v", knee.Stages[0])
	}
	if knee.Stages[1].Stage != "jogging" || *knee.Stages[1].MedianDays != 42 {
		t.Fatalf("jogging %+v", knee.Stages[1])
	}
	if knee.Stages[2].Stage != "race" || knee.Stages[2].MedianDays != nil || knee.Stages[2].Reached != 4 {
		t.Fatalf("race %+v", knee.Stages[2])
	}
}

func TestBuildInjuryStatsEmpty(t *testing.T) {
	got := buildInjuryStats(nil, nil)
	if got.Parts == nil || len(got.Parts) != 0 {
		t.Fatalf("empty stats must serialise as []: %+v", got)
	}
}
