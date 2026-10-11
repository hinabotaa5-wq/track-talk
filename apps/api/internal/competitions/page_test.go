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
