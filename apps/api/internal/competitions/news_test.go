package competitions

import (
	"strings"
	"testing"
	"time"
)

func day(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }

func knownMeets() []KnownMeet {
	end := day(7, 20)
	return []KnownMeet{
		{Key: "a", Name: "第71回県中学陸上競技大会", StartsOn: day(7, 19), EndsOn: &end, Location: "県営陸上競技場"},
		{Key: "b", Name: "第71回県高校総合体育大会陸上競技", StartsOn: day(6, 5), Location: "県営陸上競技場"},
		{Key: "c", Name: "第20回県記録会", StartsOn: day(9, 6), Location: "市民競技場"},
		{Key: "d", Name: "第21回県記録会", StartsOn: day(10, 4), Location: "市民競技場"},
	}
}

func matchedKeys(items []NewsItem) []string {
	var keys []string
	for _, m := range MatchNews(items, knownMeets()) {
		keys = append(keys, m.Meet.Key)
	}
	return keys
}

func TestMatchNewsShortAndDecoratedNames(t *testing.T) {
	got := matchedKeys([]NewsItem{
		{Title: "県中学陸上 要項掲載"},
		{Title: "【掲載】第七十一回　県高校総合体育大会陸上競技　掲載のお知らせ"},
	})
	if strings.Join(got, ",") != "a,b" {
		t.Fatalf("got %v", got)
	}
}

func TestMatchNewsShortNameNeedsAnnualNameToContainIt(t *testing.T) {
	// The annual name does not contain 県新人, and a short news name must not match by the other direction.
	if got := matchedKeys([]NewsItem{{Title: "県新人 要項"}, {Title: "県"}}); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestMatchNewsAmbiguousNameIsSkippedUnlessDateOrVenueSeparates(t *testing.T) {
	if got := matchedKeys([]NewsItem{{Title: "県記録会 要項"}}); len(got) != 0 {
		t.Fatalf("ambiguous name matched %v", got)
	}
	if got := matchedKeys([]NewsItem{{Title: "県記録会 要項", EventDate: "2026-10-04"}}); strings.Join(got, ",") != "d" {
		t.Fatalf("date should separate, got %v", got)
	}
	both := []NewsItem{{Title: "県記録会 要項", Venue: "市民競技場"}}
	if got := matchedKeys(both); len(got) != 0 {
		t.Fatalf("venue shared by both must not separate, got %v", got)
	}
}

func TestMatchNewsEditionNumberKeepsMeetsApart(t *testing.T) {
	got := matchedKeys([]NewsItem{{Title: "第21回県記録会 要項掲載"}})
	if strings.Join(got, ",") != "d" {
		t.Fatalf("got %v", got)
	}
}

func TestMatchNewsIgnoresNonMeetsAndUnknownMeets(t *testing.T) {
	got := matchedKeys([]NewsItem{
		{Title: "第71回県中学陸上競技大会 結果"},
		{Title: "県中学陸上 審判講習会"},
		{Title: "第99回県ジュニア大会 要項掲載"},
	})
	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestMatchNewsOneItemPerMeet(t *testing.T) {
	matches := MatchNews([]NewsItem{
		{Title: "県中学陸上 要項掲載", ArticleURL: "https://example.com/1"},
		{Title: "県中学陸上 要項(改訂)", ArticleURL: "https://example.com/2"},
	}, knownMeets())
	if len(matches) != 1 || matches[0].Item.ArticleURL != "https://example.com/1" {
		t.Fatalf("got %+v", matches)
	}
}

func TestNormalizeMeetNameKanjiNumbers(t *testing.T) {
	cases := map[string]string{
		"第七十一回県中学陸上":  "第71回県中学陸上",
		"第百二十回大会":     "第120回大会",
		"二〇二六年度 要項掲載": "2026年度",
		"第１２回　大会":     "第12回大会",
		"第十回記念大会":     "第10回記念大会",
	}
	for in, want := range cases {
		if got := normalizeMeetName(in); got != want {
			t.Errorf("normalizeMeetName(%q) = %q, want %q", in, got, want)
		}
	}
}

func guidelinesFrom(t *testing.T, body string) (string, bool) {
	t.Helper()
	return GuidelinesFromArticle(strings.NewReader("<html><body>"+body+"</body></html>"), "https://example.com/news/10")
}

func TestGuidelinesFromArticleUsesYoukouLink(t *testing.T) {
	link, ok := guidelinesFrom(t, `<p>開催します</p><a href="/files/entry.pdf">申込書</a><a href="/files/youkou.pdf">大会要項(PDF)</a>`)
	if !ok || link != "https://example.com/files/youkou.pdf" {
		t.Fatalf("got %q %v", link, ok)
	}
}

func TestGuidelinesFromArticleUsesArticleWhenYoukouIsOnlyText(t *testing.T) {
	link, ok := guidelinesFrom(t, `<p>要項は次のとおりです。種目 100m</p><a href="/files/entry.pdf">申込書</a>`)
	if !ok || link != "https://example.com/news/10" {
		t.Fatalf("got %q %v", link, ok)
	}
}

func TestGuidelinesFromArticleRejectsOtherDocuments(t *testing.T) {
	for _, body := range []string{
		`<a href="/a.pdf">申込書</a><a href="/b.pdf">結果</a><a href="/c.pdf">出場者一覧表</a><a href="/d.pdf">タイムテーブル</a><a href="/e.pdf">競技日程</a>`,
		`<a href="/a.pdf">要項・申込書</a>`,
		`<a href="/a.pdf">要項一覧</a>`,
		`<p>大会のお知らせです</p>`,
	} {
		if link, ok := guidelinesFrom(t, body); ok {
			t.Errorf("%s: took %q", body, link)
		}
	}
}

func TestGuidelinesFromArticleIgnoresNavigation(t *testing.T) {
	page := `<html><head><title>要項</title></head><body><nav>要項 一覧</nav><p>お知らせ</p><footer>要項</footer></body></html>`
	if link, ok := GuidelinesFromArticle(strings.NewReader(page), "https://example.com/news/10"); ok {
		t.Fatalf("took %q", link)
	}
}

func TestParseNewsAndKeepOnlyLinksOnPage(t *testing.T) {
	items, err := parseNews("```json\n{\"news\":[{\"title\":\"A\",\"article_url\":\"https://example.com/a\"},{\"title\":\"B\",\"article_url\":\"https://example.com/made-up\"}]}\n```")
	if err != nil || len(items) != 2 {
		t.Fatalf("%v %v", items, err)
	}
	page := Page{Links: []LinkRef{{URL: "https://example.com/a/"}}}
	if !linkOnPage(page, items[0].ArticleURL) || linkOnPage(page, items[1].ArticleURL) {
		t.Fatal("only the link that is on the page may be opened")
	}
}
