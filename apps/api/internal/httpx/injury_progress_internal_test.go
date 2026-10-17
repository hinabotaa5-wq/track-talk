package httpx

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

func TestBuildInjuryProgress(t *testing.T) {
	// JST では 9/1 の投稿。
	start := time.Date(2026, 8, 31, 16, 0, 0, 0, time.UTC)
	id := uuid.New()
	posts := []db.ListMyInjuryPostsRow{{ID: id, Title: "肉離れ", CreatedAt: start, BodyPart: "hamstring-strain"}}
	reports := []db.ListMyInjuryFirstReportsRow{
		{PostID: id, Stage: "rehab", FirstAt: start.Add(12*24*time.Hour + time.Hour)},
		// 他の投稿の報告は混ざらない。
		{PostID: uuid.New(), Stage: "jogging", FirstAt: start.Add(3 * 24 * time.Hour)},
	}
	stats := []db.InjuryStageStatsRow{
		{BodyPart: "hamstring-strain", Stage: "resting", Reached: 9, MedianDays: 0.2},
		{BodyPart: "hamstring-strain", Stage: "rehab", Reached: 6, MedianDays: 13.6},
		{BodyPart: "hamstring-strain", Stage: "jogging", Reached: 5, MedianDays: 30},
		{BodyPart: "hamstring-strain", Stage: "practice", Reached: 4, MedianDays: 41},
		{BodyPart: "sprain", Stage: "race", Reached: 8, MedianDays: 20},
	}
	got := buildInjuryProgress(posts, reports, stats)

	if got.MinSample != 5 || len(got.Posts) != 1 {
		t.Fatalf("unexpected shape %+v", got)
	}
	p := got.Posts[0]
	if p.BodyPartLabel != "ハムストリングの肉離れ" || p.PostedOn != "2026-09-01" || len(p.Stages) != 5 {
		t.Fatalf("post %+v", p)
	}
	want := []struct {
		stage          string
		myDays, median *int
	}{
		{"resting", ptr(0), nil},
		{"rehab", ptr(12), ptr(14)},
		{"jogging", nil, ptr(30)},
		{"practice", nil, nil}, // 報告が5人未満なので中央値は出さない。
		{"race", nil, nil},     // 他のケガの中央値は使わない。
	}
	for i, w := range want {
		s := p.Stages[i]
		if s.Stage != w.stage || !sameInt(s.MyDays, w.myDays) || !sameInt(s.MedianDays, w.median) {
			t.Fatalf("stage %d: got %s my=%v median=%v, want %+v", i, s.Stage, deref(s.MyDays), deref(s.MedianDays), w)
		}
	}
}

func TestBuildInjuryProgressEmpty(t *testing.T) {
	got := buildInjuryProgress(nil, nil, nil)
	if got.Posts == nil || len(got.Posts) != 0 {
		t.Fatalf("empty progress must serialise as []: %+v", got)
	}
}

func ptr(v int) *int { return &v }

func sameInt(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
