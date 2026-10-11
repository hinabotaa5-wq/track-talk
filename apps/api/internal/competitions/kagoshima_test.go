package competitions

import (
	"strings"
	"testing"
	"time"
)

const fixture = `<!doctype html><html><body>
<h2>2026年競技日程<br>（要項）＆ 結果等</h2>
<table>
<thead><tr><th>日</th><th>競技会</th><th>会場</th><th>要項</th><th>結果等</th></tr></thead>
<tbody>
<tr>
<td>4/18-19</td>
<td>第1回県記録会</td>
<td>白波</td>
<td><a href="https://kariku.jp/wp-content/uploads/2026/03/1kai-1.pdf">要項</a></td>
<td><a href="https://gold.jaic.org/jaic/member/kagosima/2026/result/1kai/kyougi.html">結果</a></td>
</tr>
<tr>
<td>10/4</td>
<td>第54回薩摩川内市記録会</td>
<td>薩摩川内</td>
<td><a href="https://sports-satsumasendai.jp/">薩摩川内市スポ協ホームページ</a></td>
<td></td>
</tr>
<tr>
<td>10/18</td>
<td><a href="https://kariku46.com/relay/">ロードリレーin大崎</a></td>
<td>JATC大隅</td>
<td><a href="https://kariku.jp/youkou.pdf">要項</a></td>
<td></td>
</tr>
<tr>
<td>11/2 (変更)</td>
<td>鹿児島市高校新人</td>
<td>白波</td>
<td></td>
<td></td>
</tr>
<tr>
<td>5/3-4</td>
<td>第3回県記録会<br>県選手権(長距離種目)</td>
<td>白波</td>
<td></td>
<td></td>
</tr>
<tr>
<td>1/10</td>
<td>第44回いぶすき菜の花マラソン</td>
<td>指宿市</td>
<td><a href="https://ibusuki-nanohana.com/">大会公式ホームページ</a></td>
<td></td>
</tr>
<tr>
<td>11/5</td>
<td>県中学駅伝競走大会</td>
<td>白波</td>
<td><a href="https://kagoshima-tyutairen.info/">県中体連</a></td>
<td></td>
</tr>
<tr>
<td>2/10</td>
<td>県高校新人駅伝競走大会</td>
<td>未定</td>
<td><a href="https://kagoshima-koutairen.com/yoko_shinjin_r08/">県高体連</a></td>
<td></td>
</tr>
</tbody>
</table>
</body></html>`

func TestParseKagoshima(t *testing.T) {
	meets, err := ParseKagoshima(strings.NewReader(fixture), KagoshimaSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(meets) != 8 {
		t.Fatalf("len %d", len(meets))
	}

	first := meets[0]
	if first.Name != "第1回県記録会" || first.Prefecture != "46" || first.SeasonYear != 2026 {
		t.Fatalf("first identity: %+v", first)
	}
	if !first.StartsOn.Equal(time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)) || first.EndsOn == nil || first.EndsOn.Day() != 19 {
		t.Fatalf("first dates: %s %v", first.StartsOn, first.EndsOn)
	}
	if first.OrganizerURL != "https://kariku.jp/wp-content/uploads/2026/03/1kai-1.pdf" || first.LinkLabel != "要項" {
		t.Fatalf("guidelines pdf: %s %s", first.OrganizerURL, first.LinkLabel)
	}
	if meets[1].OrganizerURL != "https://sports-satsumasendai.jp/" || meets[1].LinkLabel != "薩摩川内市スポ協ホームページ" || meets[1].EndsOn != nil {
		t.Fatalf("homepage: %+v", meets[1])
	}
	if meets[2].OrganizerURL != "https://kariku.jp/youkou.pdf" || meets[2].LinkLabel != "要項" {
		t.Fatalf("guidelines column should win over the name link: %s %s", meets[2].OrganizerURL, meets[2].LinkLabel)
	}
	if meets[3].OrganizerURL != KagoshimaSourceURL || meets[3].LinkLabel != "" {
		t.Fatalf("missing guidelines: %+v", meets[3])
	}

	changed := meets[3]
	if !changed.StartsOn.Equal(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("changed date: %s", changed.StartsOn)
	}

	combined := meets[4]
	if combined.Name != "第3回県記録会 県選手権(長距離種目)" {
		t.Fatalf("name: %q", combined.Name)
	}

	january := meets[5]
	if !january.StartsOn.Equal(time.Date(2027, 1, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("january: %s", january.StartsOn)
	}
	if january.OrganizerURL != "https://ibusuki-nanohana.com/" || january.LinkLabel != "大会公式ホームページ" {
		t.Fatalf("january homepage: %+v", january)
	}
	association := meets[6]
	if association.OrganizerURL != "https://kagoshima-tyutairen.info/" || association.LinkLabel != "県中体連" {
		t.Fatalf("association homepage: %+v", association)
	}
	guidelinesPage := meets[7]
	if guidelinesPage.OrganizerURL != "https://kagoshima-koutairen.com/yoko_shinjin_r08/" || guidelinesPage.LinkLabel != "要項" {
		t.Fatalf("guidelines page: %+v", guidelinesPage)
	}
}
