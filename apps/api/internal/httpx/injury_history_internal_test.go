package httpx

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

func TestBuildInjuryHistory(t *testing.T) {
	start := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	withReports, noReports := uuid.New(), uuid.New()
	posts := []db.ListMyInjuryPostsRow{
		{ID: withReports, Title: "疲労骨折", CreatedAt: start, BodyPart: "shin-splints"},
		{ID: noReports, Title: "肉離れ", CreatedAt: start, BodyPart: "hamstring-strain"},
	}
	reports := []db.ListMyInjuryFirstReportsRow{
		{PostID: withReports, Stage: "jogging", FirstAt: start.Add(45*24*time.Hour + 3*time.Hour)},
		{PostID: withReports, Stage: "rehab", FirstAt: start.Add(10*24*time.Hour - time.Minute)},
		{PostID: withReports, Stage: "race", FirstAt: start.Add(-time.Hour)},
	}
	got := buildInjuryHistory(posts, reports)

	if len(got.Items) != 2 {
		t.Fatalf("items %+v", got.Items)
	}
	a := got.Items[0]
	if a.BodyPartLabel != "シンスプリント" || len(a.Stages) != 3 {
		t.Fatalf("first item %+v", a)
	}
	// 段階は rehab → jogging → race の順。日数は切り捨て、負にはならない。
	if a.Stages[0].Stage != "rehab" || a.Stages[0].Days != 9 {
		t.Fatalf("rehab %+v", a.Stages[0])
	}
	if a.Stages[1].Stage != "jogging" || a.Stages[1].Days != 45 {
		t.Fatalf("jogging %+v", a.Stages[1])
	}
	if a.Stages[2].Stage != "race" || a.Stages[2].Days != 0 {
		t.Fatalf("race %+v", a.Stages[2])
	}
	if b := got.Items[1]; b.Stages == nil || len(b.Stages) != 0 {
		t.Fatalf("no reports must serialise as []: %+v", b)
	}
	if empty := buildInjuryHistory(nil, nil); empty.Items == nil {
		t.Fatalf("empty history must serialise as []")
	}
}
