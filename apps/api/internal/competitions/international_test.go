package competitions

import (
	"strings"
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/jst"
)

func TestBuildInternationalMeetsUsesDiamondLeagueSiteAndKeepsSeries(t *testing.T) {
	timetable := "/competitions/diamond-league/xiamen/timetable"
	calendar := "/competitions/world-athletics-relays/gaborone-2026/timetable"
	from := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2028, time.March, 31, 0, 0, 0, 0, time.UTC)
	meets, err := BuildInternationalMeets([]waEvent{
		{
			ID: 1, Name: "Wanda Diamond League", RankingCategory: "GW",
			CompetitionGroup: "Wanda Diamond League Meeting",
			StartDate:        "2026-05-16", EndDate: "2026-05-16",
			Venue: "China Textile City Sports Centre, Shaoxing/Keqiao (CHN)",
		},
		{
			ID: 2, Name: "Wanda Diamond League Xiamen", RankingCategory: "GW",
			CompetitionGroup: "Wanda Diamond League Meeting",
			StartDate:        "2026-05-23", EndDate: "2026-05-23",
			Venue:  "Egret Stadium, Xiamen (CHN)",
			WasURL: &timetable,
		},
		{
			ID: 3, Name: "World Athletics Relays", RankingCategory: "F",
			CompetitionGroup: "World Athletics Relays, World Athletics Series",
			StartDate:        "2026-05-02", EndDate: "2026-05-03",
			Venue:  "National Stadium, Gaborone (BOT)",
			WasURL: &calendar,
		},
		{
			ID: 4, Name: "Bank of America Chicago Marathon", RankingCategory: "GW",
			CompetitionGroup: "World Athletics Label Road Races – Platinum",
			StartDate:        "2026-10-11", Venue: "Chicago, IL (USA)",
		},
		{
			ID: 7, Name: "Prefontaine Classic", RankingCategory: "GW",
			CompetitionGroup: "Wanda Diamond League Meeting",
			StartDate:        "2026-07-03", Venue: "Hayward Field, Eugene, OR (USA)",
		},
		{
			ID: 8, Name: "Weltklasse Zürich", RankingCategory: "GW",
			CompetitionGroup: "Wanda Diamond League Meeting",
			StartDate:        "2026-08-26", Venue: "Letzigrund, Zürich (SUI)",
		},
		{
			ID: 5, Name: "National Championships Window", RankingCategory: "F",
			CompetitionGroup: "National Championships Window",
			StartDate:        "2026-06-01", Venue: "Various",
		},
		{
			ID: 6, Name: "Local Meeting", RankingCategory: "F",
			StartDate: "2026-06-02", Venue: "Somewhere",
		},
	}, []diamondMeeting{
		{
			StartsOn: time.Date(2026, time.May, 16, 0, 0, 0, 0, time.UTC),
			Location: "Shanghai/Keqiao (CHN)",
			URL:      "https://shanghai.diamondleague.com/",
		},
		{
			StartsOn: time.Date(2026, time.September, 4, 0, 0, 0, 0, time.UTC),
			EndsOn:   ptr(time.Date(2026, time.September, 5, 0, 0, 0, 0, time.UTC)),
			Location: "Brussels (BEL)",
			URL:      "https://brussels.diamondleague.com/",
		},
		{
			StartsOn: time.Date(2026, time.July, 4, 0, 0, 0, 0, time.UTC),
			Location: "Eugene (USA)",
			URL:      "https://eugene.diamondleague.com/",
		},
		{
			StartsOn: time.Date(2026, time.August, 27, 0, 0, 0, 0, time.UTC),
			Location: "Zurich (SUI)",
			URL:      "https://zurich.diamondleague.com/",
		},
	}, from, to)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Meet{}
	for _, meet := range meets {
		byName[meet.Name] = meet
	}
	if len(byName) != 7 {
		t.Fatalf("len %d %+v", len(meets), meets)
	}
	shanghai := byName["ダイヤモンドリーグ・上海/柯橋"]
	if shanghai.OrganizerURL != "https://shanghai.diamondleague.com/" || shanghai.LinkLabel != "公式サイト" || shanghai.SourceURL != diamondLeagueCalendar || shanghai.Prefecture != "00" || shanghai.SeasonYear != 2026 {
		t.Fatalf("shanghai %+v", shanghai)
	}
	if shanghai.Location != "紹興/柯橋（中国）" {
		t.Fatalf("venue %+v", shanghai)
	}
	xiamen := byName["ダイヤモンドリーグ・アモイ"]
	if xiamen.OrganizerURL != "https://worldathletics.org"+timetable || xiamen.LinkLabel != "大会ページ" || xiamen.Location != "アモイ（中国）" {
		t.Fatalf("xiamen keeps the world athletics page when the diamond league card is missing: %+v", xiamen)
	}
	relays := byName["世界リレー"]
	if relays.EndsOn == nil || relays.EndsOn.Day() != 3 || relays.OrganizerURL != "https://worldathletics.org"+calendar || relays.Location != "ハボローネ（ボツワナ）" {
		t.Fatalf("relays %+v", relays)
	}
	if _, ok := byName["National Championships Window"]; ok {
		t.Fatal("national window should be dropped")
	}
	if _, ok := byName["Local Meeting"]; ok {
		t.Fatal("domestic meeting should be dropped")
	}
	brussels := byName["ダイヤモンドリーグ・ブリュッセル"]
	if brussels.EndsOn == nil || brussels.EndsOn.Day() != 5 || brussels.OrganizerURL != "https://brussels.diamondleague.com/" || brussels.Location != "ブリュッセル（ベルギー）" {
		t.Fatalf("brussels %+v", brussels)
	}
	chicago := byName["シカゴマラソン"]
	if chicago.LinkLabel != "世界陸連のカレンダー" || chicago.OrganizerURL != worldAthleticsCalendar || chicago.StartsOn.Month() != time.October || chicago.Location != "シカゴ（アメリカ）" {
		t.Fatalf("chicago %+v", chicago)
	}
	eugene := byName["プレフォンテーンクラシック"]
	if eugene.OrganizerURL != "https://eugene.diamondleague.com/" || eugene.StartsOn.Format("2006-01-02") != "2026-07-03" || eugene.EndsOn == nil || eugene.EndsOn.Format("2006-01-02") != "2026-07-04" || eugene.Location != "ユージーン（アメリカ）" {
		t.Fatalf("eugene %+v", eugene)
	}
	zurich := byName["ヴェルトクラッセ・チューリッヒ"]
	if zurich.OrganizerURL != "https://zurich.diamondleague.com/" || zurich.StartsOn.Format("2006-01-02") != "2026-08-26" || zurich.EndsOn == nil || zurich.EndsOn.Day() != 27 || zurich.Location != "チューリッヒ（スイス）" {
		t.Fatalf("zurich %+v", zurich)
	}
	if _, ok := byName["ダイヤモンドリーグ・ユージーン"]; ok {
		t.Fatal("eugene should be merged")
	}
}

func TestParseDiamondLeagueReadsMeetingCards(t *testing.T) {
	body := `<!doctype html><html><head><title>Calendar 2026 - Wanda Diamond League</title></head><body>
<a href="https://shanghai.diamondleague.com/" class="text-sm">Shanghai/Keqiao</a>
<a href="https://shanghai.diamondleague.com/" class="group flex gap-4 border">
  <span>16</span><span>May</span><span>16 May</span><span>Shanghai/Keqiao (CHN)</span>
</a>
<a href="http://rome.diamondleague.com/" class="group flex gap-4">
  <span>04 June</span><span>Rome (ITA)</span>
</a>
<a href="https://brussels.diamondleague.com/" class="group flex gap-4">
  <span>04-05 September</span><span>Brussels (BEL)</span>
</a>
</body></html>`
	meetings, err := parseDiamondLeague([]byte(body), diamondLeagueCalendar)
	if err != nil {
		t.Fatal(err)
	}
	if len(meetings) != 3 {
		t.Fatalf("len %d %+v", len(meetings), meetings)
	}
	if meetings[0].URL != "https://shanghai.diamondleague.com/" || meetings[0].StartsOn.Format("2006-01-02") != "2026-05-16" || meetings[0].EndsOn != nil {
		t.Fatalf("shanghai %+v", meetings[0])
	}
	if meetings[1].URL != "http://rome.diamondleague.com/" || meetings[1].StartsOn.Format("2006-01-02") != "2026-06-04" {
		t.Fatalf("rome %+v", meetings[1])
	}
	if meetings[2].EndsOn == nil || meetings[2].EndsOn.Format("2006-01-02") != "2026-09-05" {
		t.Fatalf("brussels %+v", meetings[2])
	}
}

func TestParseWorldAthleticsRejectsAnUnfilteredCalendar(t *testing.T) {
	body := `<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"initialEvents":{"hits":1,"results":[],"parameters":{"startDate":null,"endDate":null,"rankingCategoryId":null,"competitionGroupId":null}}}}}</script>`
	_, _, err := parseWorldAthletics([]byte(body), time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), time.Date(2028, 3, 31, 0, 0, 0, 0, time.UTC), urlValues("rankingCategoryId", "5"))
	if err == nil || !strings.Contains(err.Error(), "期間") {
		t.Fatalf("got %v", err)
	}
}

func TestInternationalWindowCoversThisSeasonAndTheNext(t *testing.T) {
	october := time.Date(2026, time.October, 12, 5, 0, 0, 0, jst.Zone)
	from, to, seasons := InternationalWindow(october)
	if SeasonYear(october) != 2026 || seasons[0] != 2026 || seasons[1] != 2027 {
		t.Fatalf("season %+v %d", seasons, SeasonYear(october))
	}
	if from.Format("2006-01-02") != "2026-04-01" || to.Format("2006-01-02") != "2028-03-31" {
		t.Fatalf("window %s %s", from, to)
	}
	january := time.Date(2027, time.January, 2, 0, 0, 0, 0, time.UTC)
	if SeasonYear(january) != 2026 {
		t.Fatalf("january season %d", SeasonYear(january))
	}
}

func ptr(day time.Time) *time.Time { return &day }

func urlValues(key, value string) map[string][]string {
	return map[string][]string{key: {value}}
}
