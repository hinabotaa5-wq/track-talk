package competitions

import (
	"testing"
	"time"
)

func TestBuildMeetsKeepsGuidelinesOverDistrictAndDropsUnknownURLs(t *testing.T) {
	page := Page{
		SeasonYear: 2026,
		Links: []LinkRef{
			{URL: "https://example.com/yoko.pdf", Text: "要項", Header: "要項"},
			{URL: "https://example.com/result.pdf", Text: "結果", Header: "結果等"},
			{URL: "https://sports.example.jp/", Text: "市スポ協ホームページ", Header: "要項"},
			{URL: "https://koutairen.example.jp/yoko_shinjin/", Text: "県高体連", Header: "要項"},
		},
	}
	source := "https://example.com/competition/"
	meets, err := BuildMeets(page, []Draft{
		{Name: "記録会 https://example.com/info/", StartsOn: "2026-04-18", EndsOn: "2026-04-19", Location: "白波", GuidelinesURL: "https://example.com/yoko.pdf", DistrictURL: "https://sports.example.jp/"},
		{Name: "地区記録会", StartsOn: "2026-10-04", Location: "市内", GuidelinesURL: "https://example.com/result.pdf", DistrictURL: "https://sports.example.jp/"},
		{Name: "駅伝", StartsOn: "2027-02-10", Location: "未定", DistrictURL: "https://koutairen.example.jp/yoko_shinjin/"},
		{Name: "新人", StartsOn: "2026-11-02", Location: "白波"},
		{Name: "架空", StartsOn: "2026-05-01", GuidelinesURL: "https://evil.example/made-up.pdf"},
		{Name: "年度外", StartsOn: "2025-03-01"},
		{Name: "", StartsOn: "2026-06-01"},
	}, "46", source)
	if err != nil {
		t.Fatal(err)
	}
	if len(meets) != 5 {
		t.Fatalf("len %d %+v", len(meets), meets)
	}
	if meets[0].Name != "記録会" || meets[0].OrganizerURL != "https://example.com/yoko.pdf" || meets[0].LinkLabel != "要項" || meets[0].EndsOn == nil || meets[0].EndsOn.Day() != 19 {
		t.Fatalf("guidelines wins: %+v", meets[0])
	}
	if meets[1].OrganizerURL != "https://sports.example.jp/" || meets[1].LinkLabel != "市スポ協ホームページ" {
		t.Fatalf("result pdf is not a guidelines link: %+v", meets[1])
	}
	if meets[2].OrganizerURL != "https://koutairen.example.jp/yoko_shinjin/" || meets[2].LinkLabel != "要項" || meets[2].StartsOn.Year() != 2027 {
		t.Fatalf("yoko page: %+v", meets[2])
	}
	if meets[3].OrganizerURL != source || meets[3].LinkLabel != "" {
		t.Fatalf("fallback: %+v", meets[3])
	}
	if meets[4].Name != "架空" || meets[4].OrganizerURL != source {
		t.Fatalf("unknown url falls back: %+v", meets[4])
	}
	for _, meet := range meets {
		if meet.Prefecture != "46" || meet.SeasonYear != 2026 || meet.SourceURL != source {
			t.Fatalf("identity %+v", meet)
		}
	}
}

func TestBuildMeetsUsesGuidelinesColumnWhenGeminiOmitsTheURL(t *testing.T) {
	page := Page{
		SeasonYear: 2026,
		rows: []row{{
			Cells: []cell{
				{Header: "競技会", Text: "第2回南さつま記録会"},
				{Header: "要項", Links: []LinkRef{{
					URL:    "https://sites.google.com/site/minamisathumataikyo/",
					Text:   "南さつま市スポ協ホームページ",
					Header: "要項",
				}}},
			},
		}},
	}
	source := "https://kariku46.com/competition/"
	meets, err := BuildMeets(page, []Draft{
		{Name: "第2回南さつま記録会", StartsOn: "2026-10-17", Location: "加世田"},
	}, "46", source)
	if err != nil {
		t.Fatal(err)
	}
	if meets[0].OrganizerURL != "https://sites.google.com/site/minamisathumataikyo/" || meets[0].LinkLabel != "南さつま市スポ協ホームページ" {
		t.Fatalf("row link: %+v", meets[0])
	}
}

func TestBuildMeetsRejectsEmpty(t *testing.T) {
	_, err := BuildMeets(Page{SeasonYear: 2026}, []Draft{{Name: "日付なし"}}, "47", "https://example.com/")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInSeason(t *testing.T) {
	if !inSeason(time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC), 2026) {
		t.Fatal("march belongs to the season")
	}
	if inSeason(time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), 2026) {
		t.Fatal("previous march is outside")
	}
}
