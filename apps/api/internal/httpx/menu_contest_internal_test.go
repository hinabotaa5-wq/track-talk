package httpx

import (
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/jst"
)

func TestMenuContestSeasonRunsDecemberThroughMarch(t *testing.T) {
	rounds := menuContestSeason(2026)
	if len(rounds) != 8 {
		t.Fatalf("rounds = %d", len(rounds))
	}
	first, last := rounds[0], rounds[len(rounds)-1]
	if got := first.Start.Format(time.DateOnly); got != "2026-12-07" {
		t.Fatalf("first start = %s", got)
	}
	if got := first.entryEndsAt().Format(time.DateTime); got != "2026-12-17 00:00:00" {
		t.Fatalf("first entry end = %s", got)
	}
	if got := first.voteStartsAt().Weekday(); got != time.Friday {
		t.Fatalf("vote starts on %s", got)
	}
	if got := last.Start.Format(time.DateOnly); got != "2027-03-15" || last.Number != 8 {
		t.Fatalf("last = %s #%d", got, last.Number)
	}
	// 応募の締め切りが3月末を過ぎない最後の回。
	if got := last.entryEndsAt().Format(time.DateTime); got != "2027-03-25 00:00:00" {
		t.Fatalf("last entry end = %s", got)
	}
	for _, r := range rounds {
		if r.Start.Weekday() != time.Monday {
			t.Fatalf("%s is not Monday", r.Start)
		}
	}
}

func TestCurrentMenuContestRound(t *testing.T) {
	at := func(s string) time.Time {
		v, err := time.ParseInLocation(time.DateTime, s, jst.Zone)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if _, ok := currentMenuContestRound(at("2026-12-06 23:59:59")); ok {
		t.Fatal("before the season")
	}
	if got := nextMenuContestStart(at("2026-11-16 12:00:00")).Format(time.DateOnly); got != "2026-12-07" {
		t.Fatalf("next = %s", got)
	}
	r, ok := currentMenuContestRound(at("2026-12-20 22:00:00"))
	if !ok || r.Number != 1 {
		t.Fatalf("round = %+v %v", r, ok)
	}
	r, ok = currentMenuContestRound(at("2026-12-21 00:00:00"))
	if !ok || r.Number != 2 {
		t.Fatalf("second round = %+v %v", r, ok)
	}
	// 3月15日の回は3月28日まで投票が続く。
	r, ok = currentMenuContestRound(at("2027-03-28 10:00:00"))
	if !ok || r.Number != 8 {
		t.Fatalf("last round = %+v %v", r, ok)
	}
	if _, ok := currentMenuContestRound(at("2027-03-29 00:00:00")); ok {
		t.Fatal("after the season")
	}
	if got := nextMenuContestStart(at("2027-03-29 00:00:00")).Format(time.DateOnly); got != "2027-12-06" {
		t.Fatalf("next season = %s", got)
	}
	if got := menuContestPhase(r, at("2027-03-25 09:00:00"), true); got != "picking" {
		t.Fatalf("thursday phase = %s", got)
	}
	if got := menuContestPhase(r, at("2027-03-26 09:00:00"), false); got != "picking" {
		t.Fatalf("no finalists phase = %s", got)
	}
	if got := menuContestPhase(r, at("2027-03-26 09:00:00"), true); got != "voting" {
		t.Fatalf("friday phase = %s", got)
	}
}
