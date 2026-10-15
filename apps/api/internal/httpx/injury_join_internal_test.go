package httpx

import (
	"testing"
	"time"
)

func TestNormalizeInjuryJoinMonth(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	req := injuryJoinReq{
		Severity: "mild", SawDoctor: new(bool), Place: "grass", Activity: "jog",
		PreChanges: []string{"fatigue", "fatigue"}, Event: "long", Stage: "resting",
	}
	for month, ok := range map[string]bool{"2026-10": true, "2025-01": true, "2026-11": false, "10月": false} {
		req.InjuredMonth = month
		got, fields := normalizeInjuryJoin("shin-splints", req, now)
		if (len(fields) == 0) != ok {
			t.Fatalf("%s: fields %v", month, fields)
		}
		if ok && (got.InjuredOn.Day() != 1 || got.InjuredOn.Format("2006-01") != month) {
			t.Fatalf("%s stored as %s", month, got.InjuredOn)
		}
		if ok && len(got.PreChanges) != 1 {
			t.Fatalf("duplicates kept: %v", got.PreChanges)
		}
	}
}
