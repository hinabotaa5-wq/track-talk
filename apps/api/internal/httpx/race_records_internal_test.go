package httpx

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

func TestBuildRaceRecordsMarksPersonalBests(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 5, d, 0, 0, 0, 0, time.UTC) }
	wind := func(v int16) *int16 { return &v }
	rows := []db.ListRaceRecordsRow{
		// トラックは小さいほどよい。追い風参考(+2.1)は速くても自己ベストにしない。同タイムは更新ではない。
		{ID: uuid.New(), RacedOn: day(1), Event: "100m", Mark: 1110, Wind: wind(5)},
		{ID: uuid.New(), RacedOn: day(8), Event: "100m", Mark: 1085, Wind: wind(21)},
		{ID: uuid.New(), RacedOn: day(15), Event: "100m", Mark: 1110, Wind: wind(-3)},
		{ID: uuid.New(), RacedOn: day(22), Event: "100m", Mark: 1098, Wind: wind(20)},
		// フィールドは大きいほどよい。
		{ID: uuid.New(), RacedOn: day(1), Event: "shot-put", Mark: 1205},
		{ID: uuid.New(), RacedOn: day(9), Event: "shot-put", Mark: 1180},
		{ID: uuid.New(), RacedOn: day(20), Event: "shot-put", Mark: 1232},
	}
	got := buildRaceRecords(rows)

	if len(got.Events) != 2 || got.Events[0].Event != "100m" || got.Events[0].Count != 4 || !got.Events[0].Wind || got.Events[0].Kind != raceMarkTime {
		t.Fatalf("events %+v", got.Events)
	}
	if got.Events[1].Event != "shot-put" || got.Events[1].Count != 3 || got.Events[1].Wind || got.Events[1].Kind != raceMarkDistance {
		t.Fatalf("events %+v", got.Events)
	}
	want := []struct{ pb, aided bool }{
		{true, false}, {false, true}, {false, false}, {true, false},
		{true, false}, {false, false}, {true, false},
	}
	for i, w := range want {
		r := got.Records[i]
		if r.IsPB != w.pb || r.WindAided != w.aided {
			t.Fatalf("record %d (%s %d): pb=%v aided=%v, want %+v", i, r.Event, r.Mark, r.IsPB, r.WindAided, w)
		}
	}
	if got.Records[0].RacedOn != "2026-05-01" || got.Records[0].EventLabel != "100m" {
		t.Fatalf("first record %+v", got.Records[0])
	}
}

func TestBuildRaceRecordsEmpty(t *testing.T) {
	got := buildRaceRecords(nil)
	if got.Events == nil || got.Records == nil {
		t.Fatalf("empty records must serialise as []: %+v", got)
	}
}

func TestNormalizeRaceRecord(t *testing.T) {
	today := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	wind := func(v int) *int { return &v }
	cases := []struct {
		name  string
		req   raceRecordReq
		field string
	}{
		{"ok", raceRecordReq{RacedOn: "2026-10-17", Event: "100m", Mark: 1085, Wind: wind(18)}, ""},
		{"future", raceRecordReq{RacedOn: "2026-10-18", Event: "100m", Mark: 1085}, "raced_on"},
		{"bad date", raceRecordReq{RacedOn: "10/17", Event: "100m", Mark: 1085}, "raced_on"},
		{"other event", raceRecordReq{RacedOn: "2026-10-01", Event: "other", Mark: 1085}, "event"},
		{"zero mark", raceRecordReq{RacedOn: "2026-10-01", Event: "800m", Mark: 0}, "mark"},
		{"typo mark", raceRecordReq{RacedOn: "2026-10-01", Event: "long-jump", Mark: 7050}, "mark"},
		{"wind on 400m", raceRecordReq{RacedOn: "2026-10-01", Event: "400m", Mark: 4950, Wind: wind(10)}, "wind"},
		{"wind too strong", raceRecordReq{RacedOn: "2026-10-01", Event: "long-jump", Mark: 705, Wind: wind(100)}, "wind"},
	}
	for _, c := range cases {
		_, fields := normalizeRaceRecord(c.req, today)
		if c.field == "" && len(fields) != 0 {
			t.Fatalf("%s: unexpected errors %v", c.name, fields)
		}
		if c.field != "" && fields[c.field] == "" {
			t.Fatalf("%s: want error on %s, got %v", c.name, c.field, fields)
		}
	}
}

func TestRaceRoundIsOptionalFreeText(t *testing.T) {
	today := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	for _, round := range []string{"", "予選", "準決勝", "決勝", "記録会", "第2レース"} {
		params, fields := normalizeRaceRecord(raceRecordReq{RacedOn: "2026-10-25", Event: "100m", Mark: 1100, Round: round}, today)
		if len(fields) != 0 || params.Round != round {
			t.Fatalf("round=%q fields=%v", round, fields)
		}
		got := buildRaceRecords([]db.ListRaceRecordsRow{{ID: uuid.New(), Event: "100m", RacedOn: today, Mark: 1100, Round: params.Round}})
		if got.Records[0].Round != round {
			t.Fatalf("round lost: %+v", got)
		}
	}
	_, fields := normalizeRaceRecord(raceRecordReq{RacedOn: "2026-10-25", Event: "100m", Mark: 1100, Round: strings.Repeat("あ", 21)}, today)
	if fields["round"] == "" {
		t.Fatal("long round accepted")
	}
}
