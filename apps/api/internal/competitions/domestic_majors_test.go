package competitions

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const jaafListHTML = `<ul>
<li><a href="/competition/detail/2111/"><span class="compeDate">2026/9/5（土）～7（月）</span>
<p>第95回日本学生陸上競技対校選手権大会</p></a><span class="taikaicate">協力団体</span></li>
<li><a href="/competition/detail/2121/"><span class="compeDate">2026/7/5（日）</span>
<p>第74回全日本実業団対抗陸上競技選手権10000m</p></a></li>
<li><a href="/news/">お知らせ<p>対象外のリンク</p></a></li>
</ul>`

func TestParseJAAFList(t *testing.T) {
	entries, err := ParseJAAFList(strings.NewReader(jaafListHTML))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries: %+v", entries)
	}
	if entries[0].Title != "第95回日本学生陸上競技対校選手権大会" || entries[0].Path != "/competition/detail/2111/" {
		t.Fatalf("first: %+v", entries[0])
	}
}

func TestMatchDomesticMeet(t *testing.T) {
	cases := []struct {
		title string
		slug  string
	}{
		{"日清食品カップ 第42回全国小学生陸上競技交流大会", "nissin-cup"},
		{"第53回全日本中学校陸上競技選手権大会", "zenchu"},
		{"第20回U18／第57回U16陸上競技大会", "junior-u16-u18"},
		{"第42回U20日本陸上競技選手権大会", "junior-u20"},
		{"第79回全国高等学校陸上競技対校選手権大会（インターハイ）", "interhigh"},
		{"第95回日本学生陸上競技対校選手権大会", "nihon-incare"},
		{"第110回日本陸上競技選手権大会 兼 愛知・名古屋2026アジア競技大会 日本代表選手選考競技会", "nihon-championships"},
		{"第74回全日本実業団対抗陸上競技選手権大会", "jitsugyodan"},
		// Side events of the same name that must not become a card.
		{"第110回日本陸上競技選手権大会・リレー競技", ""},
		{"第110回日本陸上競技選手権大会・クロスカントリー競走", ""},
		{"第42回U20日本陸上競技選手権大会・混成競技", ""},
		{"みんなでつなごうリレーフェスティバル2026（リレフェス）第57回U16陸上競技大会・リレー競技", ""},
		{"第74回全日本実業団対抗陸上競技選手権10000m", ""},
	}
	for _, c := range cases {
		meet, ok := MatchDomesticMeet(c.title)
		if c.slug == "" {
			if ok {
				t.Errorf("%q should not match, got %s", c.title, meet.Slug)
			}
			continue
		}
		if !ok || meet.Slug != c.slug {
			t.Errorf("%q: got %q %v, want %s", c.title, meet.Slug, ok, c.slug)
		}
	}
}

func jaafDetailHTML(name, date, venue, extra string) string {
	return `<h3 class="middle">` + name + `</h3>` + extra + `
<dl class="dlList2 length7">
<dt>開催日</dt><dd>` + date + `</dd>
<dt>会場</dt><dd>
` + venue + `	</dd>
<dt>住所</dt><dd><a href="https://maps.example/">住所</a></dd>
</dl>`
}

func TestParseJAAFDetail(t *testing.T) {
	files := `<ul class="compeFile"><li><a href="/files/x.pdf">大会要項</a></li><li><a href="https://tf.zenchuu.jp/">公式HP</a></li></ul>`
	major, err := ParseJAAFDetail(strings.NewReader(jaafDetailHTML(
		"第53回全日本中学校陸上競技選手権大会", "2026年8月20日（木）～23日（日）", "山口・維新百年記念公園陸上競技場", files,
	)), "https://www.jaaf.or.jp/competition/detail/2058/")
	if err != nil {
		t.Fatal(err)
	}
	if major.Name != "第53回全日本中学校陸上競技選手権大会" || major.Venue != "山口・維新百年記念公園陸上競技場" {
		t.Fatalf("name/venue: %+v", major)
	}
	if !major.StartsOn.Equal(time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)) || !major.EndsOn.Equal(time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("span: %s %s", major.StartsOn, major.EndsOn)
	}
	if major.Year != 2026 || major.ScheduleLabel != "2026年8月20日〜23日" {
		t.Fatalf("year/label: %d %q", major.Year, major.ScheduleLabel)
	}
	if major.SiteURL != "https://tf.zenchuu.jp/" {
		t.Fatalf("site: %q", major.SiteURL)
	}
}

func TestParseJAAFDetailAcrossMonthsAndSingleDay(t *testing.T) {
	detail := "https://www.jaaf.or.jp/competition/detail/2052/"
	major, err := ParseJAAFDetail(strings.NewReader(jaafDetailHTML("インターハイ", "2026年7月30日（木）～8月5日（水）", "滋賀", "")), detail)
	if err != nil {
		t.Fatal(err)
	}
	if !major.EndsOn.Equal(time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)) || major.ScheduleLabel != "2026年7月30日〜8月5日" {
		t.Fatalf("across months: %+v", major)
	}
	if major.SiteURL != detail {
		t.Fatalf("without an official site the detail page is the link, got %q", major.SiteURL)
	}

	major, err = ParseJAAFDetail(strings.NewReader(jaafDetailHTML("日清", "2026年11月22日（日）", "東京", "")), detail)
	if err != nil {
		t.Fatal(err)
	}
	if !major.StartsOn.Equal(major.EndsOn) || major.ScheduleLabel != "2026年11月22日" {
		t.Fatalf("single day: %+v", major)
	}
}

func TestParseJAAFDetailUndecided(t *testing.T) {
	_, err := ParseJAAFDetail(strings.NewReader(jaafDetailHTML("クロカン", "調整中", "", "")), "https://www.jaaf.or.jp/competition/detail/2096/")
	if !errors.Is(err, errScheduleUndecided) {
		t.Fatalf("got %v", err)
	}
	if _, err := ParseJAAFDetail(strings.NewReader(jaafDetailHTML("大会", "来春", "", "")), "x"); err == nil || errors.Is(err, errScheduleUndecided) {
		t.Fatalf("unreadable date should be a plain error, got %v", err)
	}
}

func TestSeasonMonths(t *testing.T) {
	months := seasonMonths(2026, 2)
	if len(months) != 24 || !months[0].Equal(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)) || !months[23].Equal(time.Date(2028, 3, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("months: first %s last %s (%d)", months[0], months[len(months)-1], len(months))
	}
}
