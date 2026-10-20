package httpx

import (
	"testing"
	"time"
)

func TestNormalizeInjuryJoinDate(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	req := injuryJoinReq{
		Severity: "mild", SawDoctor: new(bool), Place: "grass", Activity: "jog",
		PreChanges: []string{"fatigue", "fatigue"}, Stage: "resting",
	}
	// 今日までの日を選べる。月だけ・先の日・10年より前は選べない。
	for day, ok := range map[string]bool{
		"2026-10-15": true, "2026-09-20": true, "2025-01-31": true,
		"2026-10-16": false, "2026-10": false, "10月": false, "2016-10-14": false,
	} {
		req.InjuredOn = day
		got, fields := normalizeInjuryJoin("shin-splints", req, now)
		if (len(fields) == 0) != ok {
			t.Fatalf("%s: fields %v", day, fields)
		}
		if ok && got.InjuredOn.Format("2006-01-02") != day {
			t.Fatalf("%s stored as %s", day, got.InjuredOn)
		}
		if ok && len(got.PreChanges) != 1 {
			t.Fatalf("duplicates kept: %v", got.PreChanges)
		}
	}
}

func TestNormalizeInjuryStageChange(t *testing.T) {
	got, fields := normalizeInjuryStageChange(injuryGroupStageReq{Stage: " jogging ", Actions: []string{"stretch", "bath", "stretch"}, Note: "  "})
	if len(fields) != 0 || got.Stage != "jogging" || len(got.Actions) != 2 || got.Note != nil || got.NotePublic {
		t.Fatalf("got %+v %v", got, fields)
	}
	got, fields = normalizeInjuryStageChange(injuryGroupStageReq{Stage: "race", Actions: []string{"nothing"}, Note: "焦らない", NotePublic: true})
	if len(fields) != 0 || got.Note == nil || *got.Note != "焦らない" || !got.NotePublic {
		t.Fatalf("got %+v %v", got, fields)
	}
	if _, fields = normalizeInjuryStageChange(injuryGroupStageReq{}); fields["stage"] == "" || fields["actions"] == "" {
		t.Fatalf("empty request should ask for stage and actions: %v", fields)
	}
}

func TestInjuryMiddleHalf(t *testing.T) {
	low, high := injuryMiddleHalf([]float64{100, 30, 60, 44, 74})
	// 並べると 30,44,60,74,100。25%と75%は44と74。
	if low != 44 || high != 74 {
		t.Fatalf("got %d-%d", low, high)
	}
	if low, _ := injuryMiddleHalf([]float64{-3, -3, -3, -3, -3}); low != 0 {
		t.Fatalf("should not go below zero: %d", low)
	}
}

// 日本時間の朝（UTC ではまだ前日）でも、今日を選べる。
func TestNormalizeInjuryJoinDateUsesJST(t *testing.T) {
	now := time.Date(2026, 10, 31, 20, 0, 0, 0, time.UTC) // 日本時間 11月1日 05:00
	req := injuryJoinReq{
		Severity: "mild", SawDoctor: new(bool), Place: "grass", Activity: "jog",
		PreChanges: []string{"fatigue"}, Stage: "resting", InjuredOn: "2026-11-01",
	}
	if _, fields := normalizeInjuryJoin("shin-splints", req, now); len(fields) != 0 {
		t.Fatalf("fields %v", fields)
	}
}
