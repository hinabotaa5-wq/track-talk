package competitions

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

const worldsFixture = `<table class="wikitable">
<tr><th>回数</th><th>開催日程</th><th>開催国 (回数)</th><th>開催都市</th><th>開催会場</th><th>参加国・地域</th><th>競技者数</th></tr>
<tr>
<td><a href="/wiki/2019年世界陸上競技選手権大会" title="2019年世界陸上競技選手権大会">17</a></td>
<td>2019年9月27日 - 10月6日</td>
<td>カタール</td>
<td>ドーハ</td>
<td>ハリーファ国際スタジアム</td>
<td>209</td><td>1972</td>
</tr>
<tr>
<td><a href="/wiki/2025年世界陸上競技選手権大会" title="2025年世界陸上競技選手権大会">20</a></td>
<td><a href="/wiki/2025年">2025年</a><a href="/wiki/9月13日">9月13日</a> - <a href="/wiki/9月21日">21日</a></td>
<td><img alt="日本の旗"> <a href="/wiki/日本" title="日本">日本</a> (3)</td>
<td>東京</td>
<td><a href="/wiki/国立競技場" title="国立競技場">国立競技場</a></td>
<td>194</td><td>1992</td>
</tr>
<tr>
<td><a href="/wiki/2027年世界陸上競技選手権大会" title="2027年世界陸上競技選手権大会">21</a></td>
<td><a href="/wiki/2027年">2027年</a><a href="/wiki/9月10日">9月10日</a> - <a href="/wiki/9月19日">19日</a></td>
<td><img alt="中華人民共和国の旗"> <a href="/wiki/中華人民共和国" title="中華人民共和国">中国</a> (2)</td>
<td>北京市</td>
<td>国家体育場</td>
<td></td><td></td>
</tr>
<tr>
<td><a class="new" href="/w/index.php?title=2029年世界陸上競技選手権大会&amp;action=edit&amp;redlink=1" title="2029年世界陸上競技選手権大会 (存在しないページ)">22</a></td>
<td>2029年9月</td>
<td>ケニア</td>
<td><a href="/wiki/ナイロビ" title="ナイロビ">ナイロビ</a></td>
<td>モイ国際スポーツセンター</td>
<td></td><td></td>
</tr>
<tr>
<td><a class="new" href="/w/index.php?title=2031年世界陸上競技選手権大会&amp;action=edit&amp;redlink=1" title="2031年世界陸上競技選手権大会 (存在しないページ)">23</a></td>
<td>2031年</td>
<td>ドイツ (3)</td>
<td>ミュンヘン</td>
<td>オリンピアシュタディオン</td>
<td></td><td></td>
</tr>
<tr><td colspan="7">日程は現地時間</td></tr>
<tr>
<td>24</td>
<td>2033年</td>
<td></td>
<td>未定</td>
<td></td>
<td></td><td></td>
</tr>
</table>`

const olympicsFixture = `<table class="wikitable">
<tr>
<th rowspan="2">回</th>
<th rowspan="2">年</th>
<th colspan="2">最終候補都市</th>
<th rowspan="2">開催都市を決定したIOC総会</th>
</tr>
<tr>
<th>開催都市</th>
<th>立候補都市</th>
</tr>
<tr>
<th>XXXIII</th>
<td><a href="/wiki/2024年パリオリンピック" title="2024年パリオリンピック">2024</a></td>
<td><img alt="フランスの旗"> <b>パリ</b></td>
<td>ロサンゼルス</td>
<td>第131次総会</td>
</tr>
<tr>
<th>XXXIV</th>
<td><a href="/wiki/2028年ロサンゼルスオリンピック" title="2028年ロサンゼルスオリンピック">2028</a><br><small><a href="/wiki/2024年・2028年の夏季オリンピックの開催地選考" title="2024年・2028年の夏季オリンピックの開催地選考">詳細</a></small></td>
<td><img alt="アメリカ合衆国の旗"> <b>ロサンゼルス</b></td>
<td></td>
<td>第131次総会 リマ（2017年9月13日）</td>
</tr>
<tr>
<th>XXXV</th>
<td><a href="/wiki/2032年ブリスベンオリンピック" title="2032年ブリスベンオリンピック">2032</a></td>
<td><img alt="オーストラリアの旗"> <b>ブリスベン</b></td>
<td>(他都市立候補なし）</td>
<td>第138次総会 東京（2021年7月21日）</td>
</tr>
<tr>
<th>XXXVI</th>
<td>2036</td>
<td>未定</td>
<td></td>
<td></td>
</tr>
</table>`

func TestParseWorldChampionshipsKeepsFutureHosts(t *testing.T) {
	hosts, err := ParseWorldChampionships(strings.NewReader(worldsFixture))
	if err != nil {
		t.Fatal(err)
	}
	upcoming := UpcomingMajorHosts(hosts, time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC))
	if len(upcoming) != 3 {
		t.Fatalf("upcoming: got %d hosts", len(upcoming))
	}
	beijing := upcoming[0]
	if beijing.City != "北京市" || beijing.Country != "中国" || beijing.Venue != "国家体育場" || beijing.Year != 2027 {
		t.Fatalf("beijing: %+v", beijing)
	}
	if !beijing.StartsOn.Equal(time.Date(2027, time.September, 10, 0, 0, 0, 0, time.UTC)) || !beijing.EndsOn.Equal(time.Date(2027, time.September, 19, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("beijing dates: %s %s", beijing.StartsOn, beijing.EndsOn)
	}
	if beijing.PageURL != "https://ja.wikipedia.org/wiki/"+url.PathEscape("2027年世界陸上競技選手権大会") || beijing.ScheduleLabel != "2027年9月10日〜19日" {
		t.Fatalf("beijing page: %+v", beijing)
	}
	nairobi := upcoming[1]
	if nairobi.City != "ナイロビ" || nairobi.PageURL != "" || nairobi.Name != "2029年世界陸上競技選手権大会" || nairobi.ScheduleLabel != "2029年9月" {
		t.Fatalf("nairobi: %+v", nairobi)
	}
	if nairobi.CityPageURL != "https://ja.wikipedia.org/wiki/"+url.PathEscape("ナイロビ") {
		t.Fatalf("nairobi city page: %s", nairobi.CityPageURL)
	}
	if !nairobi.StartsOn.Equal(time.Date(2029, time.September, 1, 0, 0, 0, 0, time.UTC)) || !nairobi.EndsOn.Equal(time.Date(2029, time.September, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("nairobi dates: %s %s", nairobi.StartsOn, nairobi.EndsOn)
	}
	munich := upcoming[2]
	if munich.City != "ミュンヘン" || munich.Country != "ドイツ" || munich.Venue != "オリンピアシュタディオン" || munich.Year != 2031 || munich.ScheduleLabel != "2031年" {
		t.Fatalf("munich: %+v", munich)
	}
	if !munich.StartsOn.Equal(time.Date(2031, time.January, 1, 0, 0, 0, 0, time.UTC)) || !munich.EndsOn.Equal(time.Date(2031, time.December, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("munich dates: %s %s", munich.StartsOn, munich.EndsOn)
	}
	for _, host := range upcoming {
		if host.City == "東京" || host.City == "未定" || host.City == "ドーハ" {
			t.Fatalf("kept a host that should be hidden: %+v", host)
		}
	}
	doha := findCity(hosts, "ドーハ")
	if !doha.StartsOn.Equal(time.Date(2019, time.September, 27, 0, 0, 0, 0, time.UTC)) || !doha.EndsOn.Equal(time.Date(2019, time.October, 6, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("doha dates: %s %s", doha.StartsOn, doha.EndsOn)
	}
}

func TestParseSummerOlympicsKeepsAwardedHosts(t *testing.T) {
	hosts, err := ParseSummerOlympics(strings.NewReader(olympicsFixture))
	if err != nil {
		t.Fatal(err)
	}
	upcoming := UpcomingMajorHosts(hosts, time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC))
	if len(upcoming) != 2 {
		t.Fatalf("upcoming: got %d hosts", len(upcoming))
	}
	losAngeles := upcoming[0]
	if losAngeles.City != "ロサンゼルス" || losAngeles.Country != "アメリカ合衆国" || losAngeles.Year != 2028 || losAngeles.Venue != "" || losAngeles.ScheduleLabel != "2028年" {
		t.Fatalf("los angeles: %+v", losAngeles)
	}
	if losAngeles.PageURL != "https://ja.wikipedia.org/wiki/"+url.PathEscape("2028年ロサンゼルスオリンピック") || strings.Contains(losAngeles.PageURL, "開催地") {
		t.Fatalf("los angeles page: %s", losAngeles.PageURL)
	}
	brisbane := upcoming[1]
	if brisbane.City != "ブリスベン" || brisbane.Country != "オーストラリア" || brisbane.Year != 2032 {
		t.Fatalf("brisbane: %+v", brisbane)
	}
	if findCity(hosts, "パリ").Year != 2024 {
		t.Fatal("paris missing from the full list")
	}
	if findCity(upcoming, "パリ").City != "" || findCity(upcoming, "未定").City != "" {
		t.Fatal("past or undecided host stayed in the upcoming list")
	}
}

func TestMajorHostsForStoreKeepsPreviousWhenEmpty(t *testing.T) {
	if _, replace := MajorHostsForStore(nil); replace {
		t.Fatal("empty parse should keep the previous rows")
	}
	next, replace := MajorHostsForStore([]MajorHost{{City: "北京市"}})
	if !replace || len(next) != 1 || next[0].City != "北京市" {
		t.Fatalf("replacement: %+v %v", next, replace)
	}
}

func TestWikiSectionAndThumbnail(t *testing.T) {
	index, err := sectionIndex([]byte(`{"parse":{"tocdata":{"sections":[{"line":"歴史","anchor":"歴史","index":"1"},{"line":"大会一覧","anchor":"大会一覧","index":"2"}]}}}`), "大会一覧")
	if err != nil || index != "2" {
		t.Fatalf("section: %q %v", index, err)
	}
	if _, err := sectionIndex([]byte(`{"parse":{"tocdata":{"sections":[]}}}`), "大会一覧"); err == nil {
		t.Fatal("missing section should fail")
	}
	image := thumbnailSource([]byte(`{"type":"standard","thumbnail":{"source":"https://upload.wikimedia.org/stadium.jpg"}}`))
	if image != "https://upload.wikimedia.org/stadium.jpg" {
		t.Fatalf("thumbnail: %s", image)
	}
	if thumbnailSource([]byte(`{"title":"なし"}`)) != "" {
		t.Fatal("missing thumbnail should be empty")
	}
	if thumbnailSource([]byte(`{"type":"disambiguation","thumbnail":{"source":"https://upload.wikimedia.org/list.jpg"}}`)) != "" {
		t.Fatal("disambiguation thumbnail should be skipped")
	}
}

func TestParseMajorHostsRejectsMissingTable(t *testing.T) {
	if _, err := ParseWorldChampionships(strings.NewReader("<p>なし</p>")); err == nil {
		t.Fatal("worlds table should be required")
	}
	if _, err := ParseSummerOlympics(strings.NewReader("<p>なし</p>")); err == nil {
		t.Fatal("olympics table should be required")
	}
}

func findCity(hosts []MajorHost, city string) MajorHost {
	for _, host := range hosts {
		if host.City == city {
			return host
		}
	}
	return MajorHost{}
}
