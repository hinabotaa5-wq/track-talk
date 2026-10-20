package httpx

import (
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

func TestParseMeetSessionsCSV(t *testing.T) {
	rows, errs := parseMeetSessionsCSV("開始,種目,ラウンド,性別\r\n2026-10-12 10:30,100m,予選,女\n\n2026/10/12 11:05\t走幅跳\t\t男子\n2026-10-12 12:00,5000m")
	if len(errs) != 0 {
		t.Fatalf("errs %v", errs)
	}
	if len(rows) != 3 {
		t.Fatalf("rows %d", len(rows))
	}
	if got := sessionLabel(rows[0].gender, rows[0].event, rows[0].round); got != "女子100m予選" {
		t.Fatalf("label %q", got)
	}
	if want := time.Date(2026, 10, 12, 11, 5, 0, 0, jst.Zone); !rows[1].startsAt.Equal(want) || rows[1].gender != "男子" || rows[1].round != "" {
		t.Fatalf("row1 %+v", rows[1])
	}

	_, errs = parseMeetSessionsCSV("2026-10-12 10:30,100m\n10時,200m\n2026-10-12 11:00,\n2026-10-12 11:30,400m,決勝,中間")
	if errs["line_2"] == "" || errs["line_3"] == "" || errs["line_4"] == "" || len(errs) != 3 {
		t.Fatalf("errs %v", errs)
	}
}

// 見出しなしで貼り付けて1行目の日時に誤記があっても、見出しとして黙って捨てずにエラーにする。
func TestParseMeetSessionsCSVDoesNotDropBadFirstRow(t *testing.T) {
	_, errs := parseMeetSessionsCSV("2026.10.12 10:30,100m,予選,女子\n2026-10-12 11:00,200m,予選,女子")
	if errs["line_1"] == "" {
		t.Fatalf("errs %v", errs)
	}
}

func TestNowRunningByMeet(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 10, 12, h, m, 0, 0, jst.Zone) }
	row := func(h, m int, gender, event, round string) db.ListMeetSessionsRow {
		return db.ListMeetSessionsRow{MeetKey: "k", Event: event, Round: round, Gender: gender, StartsAt: at(h, m)}
	}
	rows := []db.ListMeetSessionsRow{
		row(10, 30, "女子", "100m", "予選"),
		row(10, 30, "男子", "走幅跳", ""),
		row(11, 0, "男子", "100m", "予選"),
	}
	cases := []struct {
		now  time.Time
		want []string
	}{
		{at(10, 29), nil},
		{at(10, 30), []string{"女子100m予選", "男子走幅跳"}},
		{at(10, 59), []string{"女子100m予選", "男子走幅跳"}},
		{at(11, 0), []string{"男子100m予選"}},
		{at(13, 59), []string{"男子100m予選"}},
		{at(14, 0), nil},
	}
	for _, c := range cases {
		got := nowRunningByMeet(rows, c.now)["k"]
		if len(got) != len(c.want) {
			t.Fatalf("%s: got %v want %v", c.now.Format("15:04"), got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("%s: got %v want %v", c.now.Format("15:04"), got, c.want)
			}
		}
	}
}
