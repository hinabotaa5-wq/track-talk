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

func TestBuildMeetsUsesMarkedGuidelinesColumn(t *testing.T) {
	page := Page{
		SeasonYear: 2026,
		rows: []row{{
			Cells: []cell{
				{Header: "大会名（実施場所）", Text: "第1回大分市陸上競技記録会（ジェイリーススタジアム）"},
				{Header: "大会要項", Links: []LinkRef{{
					URL:    "https://www.oita-city-riku.net/",
					Text:   "○",
					Header: "大会要項",
				}}},
				{Header: "申込締切", Text: "大分市陸協"},
			},
		}},
	}
	source := "https://oita-rik.com/schedule"
	meets, err := BuildMeets(page, []Draft{
		{Name: "第1回大分市陸上競技記録会", StartsOn: "2026-04-05", Location: "ジェイリーススタジアム"},
	}, "44", source)
	if err != nil {
		t.Fatal(err)
	}
	if meets[0].OrganizerURL != "https://www.oita-city-riku.net/" || meets[0].LinkLabel != "大分市陸協" {
		t.Fatalf("row link: %+v", meets[0])
	}
}

func TestBuildMeetsSkipsMeetingsAndClinics(t *testing.T) {
	page := Page{SeasonYear: 2026}
	source := "http://www.sagarikujyo.jp/schedule/schedule_new.html"
	meets, err := BuildMeets(page, []Draft{
		{Name: "佐賀陸協理事会", StartsOn: "2026-12-05"},
		{Name: "審判講習会", StartsOn: "2027-03-14"},
		{Name: "トラック＆フィールド陸上クリニック", StartsOn: "2026-12-12"},
		{Name: "駅伝強化練習会", StartsOn: "2026-12-26"},
		{Name: "秋季記録会", StartsOn: "2026-11-07", Location: "SAGAスタジアム"},
	}, "41", source)
	if err != nil {
		t.Fatal(err)
	}
	if len(meets) != 1 || meets[0].Name != "秋季記録会" {
		t.Fatalf("meets %+v", meets)
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

func TestCleanNameAndLocationForSpacedOkayamaCells(t *testing.T) {
	cases := map[string]string{
		"（６８）県実業団対抗陸上競技選手権大会": "県実業団対抗陸上競技選手権大会",
		"（Ｒ８）県中学生陸上競技選手権大会":   "県中学生陸上競技選手権大会",
		"(2) 県陸上競技記録会":        "県陸上競技記録会",
		"男子（８９）女子（４２）県高校駅伝":   "男子（８９）女子（４２）県高校駅伝",
		"（１０）": "（１０）",
	}
	for in, want := range cases {
		if got := cleanName(in); got != want {
			t.Errorf("cleanName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := cleanLocation("倉 敷"); got != "倉敷" {
		t.Errorf("cleanLocation = %q", got)
	}
	if got := cleanLocation("ﾊﾟﾛﾏ瑞穂(愛知)"); got != "ﾊﾟﾛﾏ瑞穂(愛知)" {
		t.Errorf("cleanLocation = %q", got)
	}
	if got := cleanLocation("Alpha Beta"); got != "Alpha Beta" {
		t.Errorf("latin words keep their space: %q", got)
	}
}

// 名前が部分一致する行が複数あるときは、どの行の要項か決められないので付けない。
func TestDestinationForNameSkipsAmbiguousPartialMatches(t *testing.T) {
	link := func(url string) []cell {
		return []cell{{Header: "要項", Links: []LinkRef{{URL: url, Text: "要項", Header: "要項"}}}}
	}
	mk := func(name, url string) row {
		return row{Cells: append([]cell{{Header: "競技会", Text: name}}, link(url)...)}
	}
	page := Page{rows: []row{
		mk("(68)県選手権", "https://example.com/a"),
		mk("(68)県選手権(混成)", "https://example.com/b"),
	}}
	if url, _, ok := page.destinationForName("県選手権"); ok {
		t.Fatalf("ambiguous match returned %s", url)
	}
	one := Page{rows: []row{mk("(68)県選手権", "https://example.com/a")}}
	if url, _, ok := one.destinationForName("県選手権"); !ok || url != "https://example.com/a" {
		t.Fatalf("single match %s %v", url, ok)
	}
}
