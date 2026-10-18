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
		{"第38回出雲全日本大学選抜駅伝競走", "izumo-ekiden"},
		{"第44回全日本大学女子駅伝対校選手権大会", "univ-women-ekiden"},
		{"第58回全日本大学駅伝対校選手権大会", "univ-ekiden"},
		{"第46回全日本実業団対抗女子駅伝競走大会（クイーンズ駅伝）", "queens-ekiden"},
		{"第34回全国中学駅伝大会", "junior-high-ekiden"},
		{"男子第77回、女子第38回全国高等学校駅伝競走大会", "high-school-ekiden"},
		{"2025-2026 MGCシリーズ福岡国際マラソン2026 ～MGCシリーズ2026-27（男子G1）～", "fukuoka-marathon"},
		{"2026全日本大学女子選抜駅伝競走（富士山女子駅伝）", "fujisan-ekiden"},
		{"第71回全日本実業団対抗駅伝競走大会（ニューイヤー駅伝）", "new-year-ekiden"},
		{"皇后盃 第45回全国都道府県対抗女子駅伝競走大会", "prefectural-women-ekiden"},
		{"天皇盃 第32回全国都道府県対抗男子駅伝競走大会", "prefectural-men-ekiden"},
		{"2025-2026 MGCシリーズ第46回大阪国際女子マラソン大会 ～MGCシリーズ2026-27（女子G1）～", "osaka-women-marathon"},
		{"2025-2026 MGCシリーズ第75回別府大分毎日マラソン大会 ～MGCシリーズ2026-27（男子G1）～", "beppu-oita-marathon"},
		{"2025-2026 MGCシリーズ大阪マラソン2027 ～MGCシリーズ2026-27（男子G1・女子G2）～", "osaka-marathon"},
		{"2025-2026 MGCシリーズ東京マラソン2027 ～MGCシリーズ2026-27（男子G1・女子G1）～", "tokyo-marathon"},
		{"2025-2026 MGCシリーズ名古屋ウィメンズマラソン2027 ～MGCシリーズ2026-27（女子G1）～", "nagoya-women-marathon"},
		// 同じ名前でも、カードにしてはいけない併催の大会。
		{"第110回日本陸上競技選手権大会・リレー競技", ""},
		{"第110回日本陸上競技選手権大会・クロスカントリー競走", ""},
		{"第42回U20日本陸上競技選手権大会・混成競技", ""},
		{"みんなでつなごうリレーフェスティバル2026（リレフェス）第57回U16陸上競技大会・リレー競技", ""},
		{"第74回全日本実業団対抗陸上競技選手権10000m", ""},
		{"2027大阪ハーフマラソン", ""},
		{"東京レガシーハーフマラソン2026", ""},
		{"第30回日本学生ハーフマラソン選手権大会", ""},
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
	)), "https://www.jaaf.or.jp/competition/detail/2058/", 2026)
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
	major, err := ParseJAAFDetail(strings.NewReader(jaafDetailHTML("インターハイ", "2026年7月30日（木）～8月5日（水）", "滋賀", "")), detail, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if !major.EndsOn.Equal(time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)) || major.ScheduleLabel != "2026年7月30日〜8月5日" {
		t.Fatalf("across months: %+v", major)
	}
	if major.SiteURL != detail {
		t.Fatalf("without an official site the detail page is the link, got %q", major.SiteURL)
	}

	major, err = ParseJAAFDetail(strings.NewReader(jaafDetailHTML("日清", "2026年11月22日（日）", "東京", "")), detail, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if !major.StartsOn.Equal(major.EndsOn) || major.ScheduleLabel != "2026年11月22日" {
		t.Fatalf("single day: %+v", major)
	}
}

func TestParseJAAFDetailWithoutYear(t *testing.T) {
	detail := "https://www.jaaf.or.jp/competition/detail/2080/"
	page := jaafDetailHTML("2026全日本大学女子選抜駅伝競走（富士山女子駅伝）", "12月30日（水）予定", "静岡・富士山本宮浅間大社〜富士総合運動公園陸上競技場", "")
	major, err := ParseJAAFDetail(strings.NewReader(page), detail, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if !major.StartsOn.Equal(time.Date(2026, 12, 30, 0, 0, 0, 0, time.UTC)) || !major.EndsOn.Equal(major.StartsOn) || major.Year != 2026 {
		t.Fatalf("span: %+v", major)
	}
	if _, err := ParseJAAFDetail(strings.NewReader(page), detail, 0); err == nil {
		t.Fatal("a date without a year needs the list year")
	}
}

func TestParseJAAFDetailUndecided(t *testing.T) {
	_, err := ParseJAAFDetail(strings.NewReader(jaafDetailHTML("クロカン", "調整中", "", "")), "https://www.jaaf.or.jp/competition/detail/2096/", 2027)
	if !errors.Is(err, errScheduleUndecided) {
		t.Fatalf("got %v", err)
	}
	if _, err := ParseJAAFDetail(strings.NewReader(jaafDetailHTML("大会", "来春", "", "")), "x", 2026); err == nil || errors.Is(err, errScheduleUndecided) {
		t.Fatalf("unreadable date should be a plain error, got %v", err)
	}
}

func TestSeasonMonths(t *testing.T) {
	months := seasonMonths(2026, 2)
	if len(months) != 24 || !months[0].Equal(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)) || !months[23].Equal(time.Date(2028, 3, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("months: first %s last %s (%d)", months[0], months[len(months)-1], len(months))
	}
}

const incarePageHTML = `<table>
<tr><td>月</td><td>日</td><td>コード</td><td>大会名</td><td>会場</td><td>主催</td><td>状況</td></tr>
<tr><td>5</td><td>2~3</td><td>26635901</td><td>第1回筑波大記録突破競技会兼関東学生陸上競技対校選手権大会選考会</td><td>筑波大学（陸）</td><td>筑波大学</td><td>申請済み</td></tr>
<tr><td>5</td><td>21~24</td><td>26630102</td><td>第105回関東学生陸上競技対校選手権大会投擲種目</td><td>栃木県総合（運）多目的広場（投）</td><td><a href="https://kgrr.org/">関東学連</a></td><td>申請済み</td></tr>
<tr><td>5</td><td>21~24</td><td>26630101</td><td>2026第105回関東学生陸上競技対校選手権大会</td><td>栃木県総合（運）（陸）</td><td><a href="https://kgrr.org/">関東学連</a></td><td>申請済み</td></tr>
<tr><td>5</td><td>21~24</td><td>26630101</td><td>2026第105回関東学生陸上競技対校選手権大会</td><td>栃木県総合（運）（陸）</td><td><a href="https://kgrr.org/">関東学連</a></td><td>申請済み</td></tr>
<tr><td>5</td><td>21~24</td><td>26630101</td><td>2026第105回関東学生陸上競技対校選手権大会</td><td>カンセキ補助競技場</td><td><a href="https://kgrr.org/">関東学連</a></td><td>申請済み</td></tr>
</table>`

func TestParseRegionalIncare(t *testing.T) {
	major, err := ParseRegionalIncare(strings.NewReader(incarePageHTML), 2026, "https://www.iuau.jp/jigyo/2026/3jigyo.html")
	if err != nil {
		t.Fatal(err)
	}
	if major == nil {
		t.Fatal("no championship found")
	}
	if major.Name != "第105回関東学生陸上競技対校選手権大会" {
		t.Fatalf("name: %q", major.Name)
	}
	if !major.StartsOn.Equal(time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)) || !major.EndsOn.Equal(time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("span: %s %s", major.StartsOn, major.EndsOn)
	}
	if major.ScheduleLabel != "2026年5月21日〜24日" || major.Year != 2026 {
		t.Fatalf("label/year: %q %d", major.ScheduleLabel, major.Year)
	}
	if major.Venue != "栃木県総合運動公園陸上競技場・カンセキ補助競技場" {
		t.Fatalf("venue: %q", major.Venue)
	}
	if major.SiteURL != "https://kgrr.org/" || major.Category != DomesticStudent {
		t.Fatalf("site/category: %q %q", major.SiteURL, major.Category)
	}
}

func TestParseRegionalIncareOtherCases(t *testing.T) {
	row := func(month, days, code string) string {
		return `<table><tr><td>` + month + `</td><td>` + days + `</td><td>` + code + `</td><td>第1回テスト学生陸上競技対校選手権大会</td><td>会場</td><td>学連</td><td>x</td></tr></table>`
	}
	// 中止になった大会には月も日もないので、出す回がない。
	if major, _ := ParseRegionalIncare(strings.NewReader(row("中止", "中止", "26610101")), 2026, "p"); major != nil {
		t.Fatalf("cancelled: %+v", major)
	}
	// 1月から3月は、前の4月に始まった年度に入る。
	major, _ := ParseRegionalIncare(strings.NewReader(row("3", "7", "26610101")), 2026, "p")
	if major == nil || !major.StartsOn.Equal(time.Date(2027, 3, 7, 0, 0, 0, 0, time.UTC)) || major.ScheduleLabel != "2027年3月7日" {
		t.Fatalf("march: %+v", major)
	}
	if major, _ := ParseRegionalIncare(strings.NewReader(row("5", "1", "26600101")), 2026, "p"); major != nil {
		t.Fatalf("code of no federation: %+v", major)
	}
}

func TestBaseMeetName(t *testing.T) {
	cases := map[string]string{
		"第111回日本陸上競技選手権大会 兼 北京2027世界陸上競技選手権大会 日本代表選手選考競技会":         "第111回日本陸上競技選手権大会",
		"第110回日本陸上競技選手権大会 兼 愛知・名古屋2026アジア競技大会 日本代表選手選考競技会":         "第110回日本陸上競技選手権大会",
		"第53回全日本中学校陸上競技選手権大会":                                      "第53回全日本中学校陸上競技選手権大会",
		"2025-2026 MGCシリーズ東京マラソン2027 ～MGCシリーズ2026-27（男子G1・女子G1）～":  "東京マラソン2027",
		"2025-2026 MGCシリーズ第46回大阪国際女子マラソン大会 ～MGCシリーズ2026-27（女子G1）～": "第46回大阪国際女子マラソン大会",
	}
	for raw, want := range cases {
		if got := baseMeetName(raw); got != want {
			t.Errorf("%q: got %q, want %q", raw, got, want)
		}
	}
}
