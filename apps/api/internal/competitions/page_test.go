package competitions

import (
	"strings"
	"testing"
)

const scheduleHTML = `<!doctype html><html><body>
<h2>2026年競技日程<br>（要項）＆ 結果等</h2>
<table>
<thead><tr><th>日</th><th>競技会</th><th>会場</th><th>要項</th><th>結果等</th></tr></thead>
<tbody>
<tr>
<td>4/18-19</td>
<td>第1回県記録会</td>
<td>白波</td>
<td><a href="https://kariku.jp/wp-content/uploads/2026/03/1kai-1.pdf">要項</a></td>
<td><a href="https://gold.jaic.org/result.html">結果</a></td>
</tr>
<tr>
<td>10/4</td>
<td>第54回薩摩川内市記録会</td>
<td>薩摩川内</td>
<td><a href="https://sports-satsumasendai.jp/">薩摩川内市スポ協ホームページ</a></td>
<td></td>
</tr>
<tr>
<td>2/10</td>
<td>県高校新人駅伝競走大会</td>
<td>未定</td>
<td><a href="https://kagoshima-koutairen.com/yoko_shinjin_r08/">県高体連</a></td>
<td></td>
</tr>
<tr>
<td>11/2</td>
<td>鹿児島市高校新人</td>
<td>白波</td>
<td></td>
<td></td>
</tr>
</tbody>
</table>
</body></html>`

func TestParsePage(t *testing.T) {
	page, err := ParsePage(strings.NewReader(scheduleHTML), "https://kariku46.com/competition/")
	if err != nil {
		t.Fatal(err)
	}
	if page.SeasonYear != 2026 {
		t.Fatalf("season %d", page.SeasonYear)
	}
	if !strings.Contains(page.Body, "第1回県記録会") || !strings.Contains(page.Body, "https://kariku.jp/wp-content/uploads/2026/03/1kai-1.pdf") {
		t.Fatalf("body missing schedule: %s", page.Body)
	}
	if !strings.Contains(page.Body, "https://gold.jaic.org/result.html") {
		t.Fatalf("result link should stay in the text so it can be rejected later: %s", page.Body)
	}
	guidelines := 0
	for _, link := range page.Links {
		if link.Header == "要項" {
			guidelines++
		}
	}
	if guidelines != 3 {
		t.Fatalf("guidelines links %d", guidelines)
	}
}

func TestParsePageScheduleList(t *testing.T) {
	html := `<html><body><h2>2026年度</h2><ul>
<li><span><strong>第52回熊日学童オリンピック</strong><br>[ 開催予定 ] 開催:2026年10月17日（土） 会場:えがお健康スタジアム</span></li>
<li><a href="http://kumariku.com/competition/?id=9599"><strong>Ｒ８年 第５１回甲佐１０マイルロードレース</strong><br>開催: 会場:甲佐コース</a></li>
<li><a href="http://kumariku.com/competition/?id=9811"><strong>令和８年度秋季熊本陸協記録会</strong><br>開催:2026年10月24日 会場:えがお健康スタジアム</a></li>
</ul></body></html>`
	page, err := ParsePage(strings.NewReader(html), "http://kumariku.com/schedule/")
	if err != nil {
		t.Fatal(err)
	}
	if page.SeasonYear != 2026 {
		t.Fatalf("season %d", page.SeasonYear)
	}
	if strings.Contains(page.Body, "甲佐") {
		t.Fatalf("undated meet should be omitted: %s", page.Body)
	}
	if !strings.Contains(page.Body, "第52回熊日学童オリンピック") || !strings.Contains(page.Body, "2026年10月24日") {
		t.Fatalf("body %s", page.Body)
	}
	if len(page.Links) != 0 {
		t.Fatalf("list items are not guidelines links: %+v", page.Links)
	}
}

func TestParsePageKeepsCurrentSeasonTable(t *testing.T) {
	html := `<html><body>
<p><b><span>２０２６年度大会</span></b></p>
<table><tr><th>月</th><th>日</th><th>大　会　名　（　実施場所　）</th><th>大会要項</th><th>申込締切</th></tr>
<tr><td>4</td><td>5</td><td>第1回大分市陸上競技記録会（ジェイリーススタジアム）</td><td><a href="https://www.oita-city-riku.net/">○</a></td><td>大分市陸協</td></tr>
<tr><td>4</td><td>11</td><td>第1回春季陸上競技大会（ジェイリーススタジアム）</td><td><a href="https://oita-rik.com/wp/a.pdf">○</a></td><td>4/3</td></tr>
</table>
<p>２０２５年度大会</p>
<table><tr><th>月</th><th>日</th><th>大会名</th><th>要項</th></tr>
<tr><td>4</td><td>12</td><td>昨年の大会</td><td><a href="https://example.com/old.pdf">〇</a></td></tr>
</table>
</body></html>`
	page, err := ParsePage(strings.NewReader(html), "https://oita-rik.com/schedule")
	if err != nil {
		t.Fatal(err)
	}
	if page.SeasonYear != 2026 {
		t.Fatalf("season %d", page.SeasonYear)
	}
	if strings.Contains(page.Body, "昨年の大会") || !strings.Contains(page.Body, "第1回大分市陸上競技記録会") {
		t.Fatalf("body %s", page.Body)
	}
	if !strings.Contains(page.Body, "大会名（実施場所）") {
		t.Fatalf("header %s", page.Body)
	}
}

func TestParsePageUsesMonthHeading(t *testing.T) {
	html := `<html><body><h2>2026年度福岡陸上競技日程</h2>
<h3>2026年11月</h3>
<table><tr><th>開催日</th><th>競技会・行事</th><th>会場</th><th>要項</th></tr>
<tr><td>19(土）～21(月)</td><td>高校新人</td><td>博多の森</td><td><a href="https://www.fukuriku.com/a.pdf">要項</a></td></tr>
<tr><td>31(土）～11/01(日)</td><td>秋季記録会</td><td>博多の森</td><td><a href="https://www.fukuriku.com/b.pdf">要項</a></td></tr>
</table>
<h3>2026年4月</h3>
<table><tr><th>開催日</th><th>競技会</th><th>要項</th></tr>
<tr><td>4月5日</td><td>月つき</td><td><a href="https://www.fukuriku.com/c.pdf">要項</a></td></tr>
</table>
</body></html>`
	page, err := ParsePage(strings.NewReader(html), "https://www.fukuriku.com/tournament/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page.Body, "2026年11月 19(土）～21(月)") || !strings.Contains(page.Body, "2026年11月 31(土）～11/01(日)") {
		t.Fatalf("month missing: %s", page.Body)
	}
	if strings.Contains(page.Body, "2026年4月 4月5日") {
		t.Fatalf("month doubled: %s", page.Body)
	}
}

func TestParsePageSplitColumnsAndRowspan(t *testing.T) {
	html := `<html><body><h2>２０２６年度</h2><table>
<th colspan="2">日程</th><th>佐賀陸上競技協会主催</th><th>地域陸協・各郡市町村主催</th>
<tr><td rowspan="2">２０２６年０４月</td><td>０４（土）</td><td>第1回記録会（SAGAスタジアム）</td><td><br></td></tr>
<tr><td>１１（土）</td><td>春季記録会（SAGAスタジアム）<br>唐津記録会（唐津）</td><td>鳥栖大会（鳥栖）</td></tr>
</table></body></html>`
	page, err := ParsePage(strings.NewReader(html), "http://www.sagarikujyo.jp/schedule/schedule_new.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"2026年04月", "第1回記録会（SAGAスタジアム）", "春季記録会（SAGAスタジアム）", "唐津記録会（唐津）", "鳥栖大会（鳥栖）"} {
		if !strings.Contains(page.Body, want) {
			t.Fatalf("missing %s in %s", want, page.Body)
		}
	}
	if strings.Count(page.Body, "---\n") != 4 {
		t.Fatalf("rows %d\n%s", strings.Count(page.Body, "---\n"), page.Body)
	}
}

func TestParsePageMonthHeaderAndDateColumn(t *testing.T) {
	html := `<html><body><h2>2026年度 日程表</h2><table>
<tr><th>月</th><th>期日</th><th>大会名</th><th>会場</th></tr>
<tr><th rowspan="2">4月</th>
<td>5(日)14:00～</td><td>県陸協主催審判講習会</td><td>諫早
<tr><td>18(土)</td><td>ながさきリレーカーニバル</td><td>諫早</td></tr>
<tr><th>1月</th><td>23(土)</td><td>県高校新人駅伝</td><td>諫早</td></tr>
</table></body></html>`
	page, err := ParsePage(strings.NewReader(html), "https://jaaf-nagasaki.net/schedule/2026ken.htm")
	if err != nil {
		t.Fatal(err)
	}
	if page.SeasonYear != 2026 {
		t.Fatalf("season %d", page.SeasonYear)
	}
	for _, want := range []string{"月: 4月", "期日: 5(日)", "期日: 18(土)", "月: 1月", "県高校新人駅伝", "会場: 諫早"} {
		if !strings.Contains(page.Body, want) {
			t.Fatalf("missing %s in %s", want, page.Body)
		}
	}
	if strings.Contains(page.Body, "14:00") {
		t.Fatalf("clock left in date: %s", page.Body)
	}
	if strings.Count(page.Body, "月: 4月") != 2 {
		t.Fatalf("month not repeated: %s", page.Body)
	}
}

func TestParsePageFullwidthYear(t *testing.T) {
	html := `<html><body><h2>２０２６年度</h2><table><tr><th>開催日</th><th>競技会</th><th>要項</th></tr>
<tr><td>4月25日</td><td>記録会</td><td><a href="https://example.com/a.pdf">要項</a></td></tr></table></body></html>`
	page, err := ParsePage(strings.NewReader(html), "https://jaaf-okinawa.com/user.php?CMD=1")
	if err != nil {
		t.Fatal(err)
	}
	if page.SeasonYear != 2026 {
		t.Fatalf("season %d", page.SeasonYear)
	}
	if len(page.Links) != 1 || page.Links[0].URL != "https://example.com/a.pdf" {
		t.Fatalf("links %+v", page.Links)
	}
}

func TestParsePageHeadlessTable(t *testing.T) {
	html := `<html><body>
<h2>2026年度大会・行事</h2>
<h2>4月</h2>
<table><tbody>
<tr><td>4</td><td>土</td><td>審判講習会</td><td>小瀬</td><td></td></tr>
<tr><td>5,12</td><td>日,日</td><td><a href="https://example.com/record">第57回県記録会</a></td><td>小瀬</td><td>Web申込</td></tr>
</tbody></table>
<h2>5月</h2>
<table><tbody>
<tr><td>2</td><td>土</td><td>県中学普及記録会</td><td>小瀬</td><td></td></tr>
</tbody></table>
</body></html>`
	page, err := ParsePage(strings.NewReader(html), "https://example.com/2026nendo-taikai")
	if err != nil {
		t.Fatal(err)
	}
	if page.SeasonYear != 2026 {
		t.Fatalf("season %d", page.SeasonYear)
	}
	for _, want := range []string{"4月 4", "審判講習会", "4月 5,12", "第57回県記録会", "5月 2", "県中学普及記録会"} {
		if !strings.Contains(page.Body, want) {
			t.Fatalf("missing %s in %s", want, page.Body)
		}
	}
}

func TestParsePageTheadTdWithTbodyTh(t *testing.T) {
	html := `<html><body>
<p>2026年4月</p>
<table summary="日程">
<thead>
  <tr><td colspan="2">日／曜</td><td>内　容</td></tr>
</thead>
<tbody>
  <tr><th>4</th><td>土</td><td><a href="https://example.com/yoko.pdf">春季記録会</a></td></tr>
  <tr><th>5</th><td>日</td><td></td></tr>
  <tr><th>18</th><td>土</td><td>県選手権大会</td></tr>
</tbody>
</table>
</body></html>`
	page, err := ParsePage(strings.NewReader(html), "https://example.com/schedule?m=4&y=2026")
	if err != nil {
		t.Fatal(err)
	}
	if page.SeasonYear != 2026 {
		t.Fatalf("season %d", page.SeasonYear)
	}
	for _, want := range []string{"2026年4月 4", "春季記録会", "2026年4月 18", "県選手権大会"} {
		if !strings.Contains(page.Body, want) {
			t.Fatalf("missing %s in %s", want, page.Body)
		}
	}
	if strings.Contains(page.Body, "2026年4月 5") {
		t.Fatalf("empty row should not be parsed: %s", page.Body)
	}
}

func TestParsePageYearHeader(t *testing.T) {
	html := `<html><body>
<h1>2026年度競技日程</h1>
<h2>2026年4月競技日程</h2>
<table>
<thead><tr><th>2026年</th><th>競技会名</th><th>会場</th><th>要項</th></tr></thead>
<tbody>
<tr><td>4(土)</td><td>春季トライアル</td><td>佐久</td><td><a href="https://example.com/yoko.pdf">要項</a></td></tr>
</tbody>
</table>
</body></html>`
	page, err := ParsePage(strings.NewReader(html), "https://example.com/calendar/2026nrk_calendar-2-2-2-2-2/")
	if err != nil {
		t.Fatal(err)
	}
	if page.SeasonYear != 2026 {
		t.Fatalf("season %d", page.SeasonYear)
	}
	for _, want := range []string{"2026年4月 4(土)", "春季トライアル", "佐久"} {
		if !strings.Contains(page.Body, want) {
			t.Fatalf("missing %s in %s", want, page.Body)
		}
	}
}

