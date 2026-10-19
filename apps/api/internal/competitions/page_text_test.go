package competitions

import (
	"net/url"
	"strings"
	"testing"
)

func textSchedulePage() string {
	var b strings.Builder
	b.WriteString(`<html><body><nav>メニュー 12/31</nav><h2>2026（令和8）年度 すべての大会</h2>
<p><a href="/old/">2025年度の情報はこちら</a></p>`)
	for i := 1; i <= 12; i++ {
		b.WriteString(`<div><p><strong>`)
		b.WriteString(`4/` + string(rune('0'+i%9+1)) + `（土）</strong></p><div><p><strong>第` + string(rune('0'+i%9+1)) + `回記録会</strong></p><p><a href="/yoko/` + string(rune('a'+i)) + `.pdf">要項</a></p></div></div>`)
	}
	b.WriteString(`<footer>フッター 5/5</footer></body></html>`)
	return b.String()
}

func TestParsePageFallsBackToPlainTextWhenThereIsNoTable(t *testing.T) {
	page, err := ParsePage(strings.NewReader(textSchedulePage()), "https://example.com/competition/")
	if err != nil {
		t.Fatal(err)
	}
	if !page.plainText {
		t.Fatal("expected plain text page")
	}
	if page.SeasonYear != 2026 {
		t.Fatalf("season %d: the newest season named wins over a link to last season", page.SeasonYear)
	}
	if strings.Contains(page.Body, "メニュー") || strings.Contains(page.Body, "フッター") {
		t.Fatalf("nav and footer are dropped: %s", page.Body)
	}
	if !strings.Contains(page.Body, "要項 https://example.com/yoko/b.pdf") {
		t.Fatalf("link keeps its text and absolute url: %s", page.Body)
	}
	found := false
	for _, link := range page.Links {
		if link.URL == "https://example.com/yoko/b.pdf" && link.Text == "要項" {
			found = true
		}
	}
	if !found {
		t.Fatalf("links: %+v", page.Links)
	}
	if !strings.Contains(extractionPrompt(page), "ページの文字を行ごとに並べた") {
		t.Fatal("plain text note is added to the prompt")
	}
}

func TestParsePageStillRejectsPagesWithoutManyDates(t *testing.T) {
	_, err := ParsePage(strings.NewReader(`<html><body><h1>2026年度</h1><p>お知らせ 4/1 5/2</p></body></html>`), "https://example.com/")
	if err == nil {
		t.Fatal("a news page is not a schedule")
	}
}

func TestIsCompetitionNameDropsCancelledMeets(t *testing.T) {
	if isCompetitionName("山口県中学校新人陸上競技記録会【中止】") {
		t.Fatal("cancelled meet is kept")
	}
	if !isCompetitionName("山口県中学校新人陸上競技記録会") {
		t.Fatal("normal meet is dropped")
	}
}

func TestCalendarWidgetIgnored(t *testing.T) {
	html := `<html><body>
<h1>2026年度</h1>
<table id="wp-calendar" class="wp-calendar-table">
<thead><tr><th>月</th><th>火</th><th>水</th><th>木</th><th>金</th><th>土</th><th>日</th></tr></thead>
<tbody><tr><td>1</td><td>2</td><td>3</td><td>4</td><td>5</td><td>6</td><td>7</td></tr></tbody>
</table>
<table id="tablepress-1">
<thead><tr><th>日</th><th>大会名</th></tr></thead>
<tbody><tr><td>4/18</td><td>記録会</td></tr></tbody>
</table>
</body></html>`
	page, err := ParsePage(strings.NewReader(html), "https://example.com/")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(page.Body, "wp-calendar") || len(page.rows) != 1 {
		t.Fatalf("calendar widget must be ignored, got rows=%d body=%s", len(page.rows), page.Body)
	}
}

func TestYearFromURL(t *testing.T) {
	u, _ := url.Parse("https://www.oaaa.jp/competition/?y=2026")
	if y, ok := yearFromURL(u); !ok || y != 2026 {
		t.Fatalf("expected 2026, got %d, %v", y, ok)
	}
	u2, _ := url.Parse("https://jaaf-nagasaki.net/schedule/2026ken.htm")
	if y, ok := yearFromURL(u2); !ok || y != 2026 {
		t.Fatalf("expected 2026, got %d, %v", y, ok)
	}
	// 月ごとのページの1〜3月は、前の年の競技年度。
	for _, raw := range []string{
		"https://www.fukui-jaaf.com/020_schedule/index.php?m=12&y=2026",
		"https://www.fukui-jaaf.com/020_schedule/index.php?m=1&y=2027",
		"https://www.fukui-jaaf.com/020_schedule/index.php?m=3&y=2027",
	} {
		u, _ := url.Parse(raw)
		if y, ok := yearFromURL(u); !ok || y != 2026 {
			t.Fatalf("%s: expected 2026, got %d, %v", raw, y, ok)
		}
	}
}
