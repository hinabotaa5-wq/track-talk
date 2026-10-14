package competitions

import "context"

// Sources are the athletics association homepages to read.
// Other prefectures are added here as their schedule page URL is known.
var Sources = []Source{
	{Prefecture: "01", PageURL: "https://hokkaido-rikkyo.jp/schedule/index.html", PDFLinks: []string{`北海道内全競技会日程|schedule_hokkaido`}, NewsURL: "https://hokkaido-rikkyo.jp/information/index.html"},
	{Prefecture: "02", PageURL: "https://www.jaafaomori.com/schedule/", NewsURL: "https://www.jaafaomori.com/news/"},
	{Prefecture: "03", PageURL: "https://iwate-rk.com/competition-info/", NewsURL: "https://iwate-rk.com/"},
	{
		Prefecture: "04",
		NewsURL:    "https://www.jaaf-miyagi.com/",
		PageURL:    "https://www.jaaf-miyagi.com/schedule/",
		PDFLinks:   []string{`主催・主管・協力大会`},
	},
	{Prefecture: "05", PageURL: "http://jaaf-akita.com/calendar", NewsURL: "http://jaaf-akita.com/news/"},
	{Prefecture: "06", PageURL: "https://jaaf-yamagata.jp/", PDFLinks: []string{`主要競技会日程|kyouginittei`}, NewsURL: "https://jaaf-yamagata.jp/taikai/index.html"},
	{Prefecture: "07", PageURL: "https://gold.jaic.org/fukushima/joho/2026/2026Sche.pdf", NewsURL: "https://gold.jaic.org/fukushima/joho.html"},
	{Prefecture: "08", PageURL: "http://ibariku.com/event/2026.html", NewsURL: "https://ibariku.com/"},
	{Prefecture: "09", PageURL: "https://www.jaaftochigi.jp/event2026/", NewsURL: "https://www.jaaftochigi.jp/"},
	{Prefecture: "10", PageURL: "http://gold.jaic.org/gunma/competitions.html", NewsURL: "http://gold.jaic.org/gunma/competitions.html"},
	{
		Prefecture: "11",
		NewsURL:    "http://sairiku.net/bace/news/",
		PageURL:    "http://sairiku.net/bace/Event/R08Event/EventR0804",
		ExtraURLs: []string{
			"http://sairiku.net/bace/Event/R08Event/EventR0805",
			"http://sairiku.net/bace/Event/R08Event/EventR0806",
			"http://sairiku.net/bace/Event/R08Event/EventR0807",
			"http://sairiku.net/bace/Event/R08Event/EventR0808",
			"http://sairiku.net/bace/Event/R08Event/EventR0809",
			"http://sairiku.net/bace/Event/R08Event/EventR0810",
			"http://sairiku.net/bace/Event/R08Event/EventR0811",
		},
	},
	{Prefecture: "12", PageURL: "https://www.jaaf-chiba.jp/", PDFLinks: []string{`_schedule_hp`}, NewsURL: "http://www.jaaf-chiba.jp/pages/main/taikai/index.html"},
	{Prefecture: "13", PageURL: "https://toriku.or.jp/competition/", NewsURL: "https://toriku.or.jp/news/"},
	{Prefecture: "14", PageURL: "https://www.kanagawariku.org/kana21cal100.html", PDFLinks: []string{`公認競技会`}, NewsURL: "https://www.kanagawariku.org/kana31com100.html"},
	{Prefecture: "15", PageURL: "https://nrkk.net/official-eventschedule", PDFLinks: []string{`公認競技会`}, NewsURL: "https://nrkk.net/news/"},
	{Prefecture: "16", PageURL: "https://www.jaaf-toyama.net/schedule/", PDFLinks: []string{`行事日程`}, NewsURL: "https://www.jaaf-toyama.net/news/"},
	{Prefecture: "17", PageURL: "https://ishiriku.jp/schedule.html", PDFLinks: []string{`石川陸上競技協会.*競技日程`}, NewsURL: "https://ishiriku.jp/"},
	{
		Prefecture: "18",
		NewsURL:    "https://www.fukui-jaaf.com/information/",
		PageURL:    "https://www.fukui-jaaf.com/020_schedule/index.php?m=4&y=2026",
		ExtraURLs: []string{
			"https://www.fukui-jaaf.com/020_schedule/index.php?m=5&y=2026",
			"https://www.fukui-jaaf.com/020_schedule/index.php?m=6&y=2026",
			"https://www.fukui-jaaf.com/020_schedule/index.php?m=7&y=2026",
			"https://www.fukui-jaaf.com/020_schedule/index.php?m=8&y=2026",
			"https://www.fukui-jaaf.com/020_schedule/index.php?m=9&y=2026",
			"https://www.fukui-jaaf.com/020_schedule/index.php?m=10&y=2026",
			"https://www.fukui-jaaf.com/020_schedule/index.php?m=11&y=2026",
			"https://www.fukui-jaaf.com/020_schedule/index.php?m=12&y=2026",
			"https://www.fukui-jaaf.com/020_schedule/index.php?m=1&y=2027",
			"https://www.fukui-jaaf.com/020_schedule/index.php?m=2&y=2027",
			"https://www.fukui-jaaf.com/020_schedule/index.php?m=3&y=2027",
		},
	},
	{Prefecture: "19", PageURL: "https://yamanashitf.com/2026nendo-taikai", NewsURL: "https://yamanashitf.com/archives/category/topics"},
	{Prefecture: "20", PageURL: "https://nagano-rk.com/calendar/2026nrk_calendar-2-2-2-2-2-2/", NewsURL: "https://nagano-rk.com/"},
	{
		Prefecture: "22",
		PageURL:    "https://shizuoka-jaaf.com/srkkyogi.html",
		ExtraURLs:  []string{"https://seiburiku.jp/schedule/"},
	},
	{
		Prefecture: "23",
		NewsURL:    "http://aichi-rk.jp/",
		PageURL:    "http://aichi-rk.jp/01nittei.htm",
		ExtraURLs: []string{
			"http://aichi-rk.jp/01_01nittei.htm",
			"http://aichi-rk.jp/01_02nittei.htm",
			"http://aichi-rk.jp/01_03nittei.htm",
		},
	},
	{Prefecture: "21", PageURL: "https://www.gifu-riku.com/schedule/", NewsURL: "https://www.gifu-riku.com/news/"},
	{Prefecture: "24", PageURL: "https://mierk.jp/taikaiyoukou/", PDFLinks: []string{`県内日程表`}, NewsURL: "https://mierk.jp/"},
	{Prefecture: "25", PageURL: "https://srkshiga.com/?page_id=1036", NewsURL: "https://srkshiga.com/"},
	{Prefecture: "26", PageURL: "https://www.krk26.jp/", PDFLinks: []string{`競技会日程`}, NewsURL: "https://www.krk26.jp/index.php?belong2=一般競技会"},
	{Prefecture: "27", PageURL: "https://www.oaaa.jp/competition/?y=2026", NewsURL: "https://www.oaaa.jp/news/"},
	{Prefecture: "28", PageURL: "http://haaa.jp/taikai.html", PDFLinks: []string{`競技会日程表`}, NewsURL: "http://haaa.jp/info.html"},
	{Prefecture: "29", PageURL: "http://www.narariku.com/information.html", NewsURL: "http://www.narariku.com/information.html"},
	{Prefecture: "30", PageURL: "https://wariku.com/athletics-list/", NewsURL: "https://wariku.com/news/"},
	{Prefecture: "31", PageURL: "https://jaaf-tottori.com/competition/", NewsURL: "https://jaaf-tottori.com/information/"},
	{Prefecture: "32", PageURL: "https://www.shimariku.jp/record", PDFLinks: []string{`陸協行事予定表`}, NewsURL: "https://www.shimariku.jp/news"},
	{Prefecture: "33", PageURL: "https://okariku33.com/?page_id=1291", NewsURL: "https://okariku33.com/?page_id=15"},
	{Prefecture: "34", PageURL: "https://hiroshimatf.org/competitionandschedule/", PDFLinks: []string{`競技会日程`}, NewsURL: "https://hiroshimatf.org/news"},
	{Prefecture: "35", PageURL: "https://yaaf.jp/kyougikai-top/schedule-all/", NewsURL: "https://yaaf.jp/category/oshirase/"},
	{Prefecture: "36", PageURL: "https://jaaftokushima.com/wp/schedule_table26/", NewsURL: "https://jaaftokushima.com/wp/"},
	{Prefecture: "37", PageURL: "https://gold.jaic.org/kagawa/2026/2026info.html", NewsURL: "https://gold.jaic.org/kagawa/2026/2026info.html"},
	{Prefecture: "38", PageURL: "https://www.ehime-rikujyo.jp/01_taikai.html", NewsURL: "https://ehime-rikujyo.jp/"},
	{Prefecture: "39", PageURL: "https://npo-kochi.sports.coocan.jp/kyogi.files/sheet001.htm", NewsURL: "https://npo-kochi.sports.coocan.jp/index.files/sheet001.htm"},
	{Prefecture: "40", PageURL: "https://www.fukuriku.com/tournament/", NewsURL: "https://www.fukuriku.com/topics/"},
	{Prefecture: "41", PageURL: "http://www.sagarikujyo.jp/schedule/schedule_new.html", NewsURL: "http://www.sagarikujyo.jp/"},
	{Prefecture: "42", PageURL: "https://jaaf-nagasaki.net/schedule/2026ken.htm", NewsURL: "https://jaaf-nagasaki.net/"},
	{
		Prefecture: "43",
		NewsURL:    "http://kumariku.com/info/",
		PageURL:    "http://kumariku.com/schedule/",
		ExtraURLs:  []string{"http://kumariku.com/result/?y=2026"},
	},
	{Prefecture: "44", PageURL: "https://oita-rik.com/schedule", NewsURL: "https://oita-rik.com/newstopics"},
	{Prefecture: "45", PageURL: "https://www.miyariku.org/maaf/2026/03/05/2026%E5%B9%B4%E5%BA%A6%E7%AB%B6%E6%8A%80%E4%BC%9A%E6%83%85%E5%A0%B1/", NewsURL: "https://www.miyariku.org/"},
	{Prefecture: "46", PageURL: "https://kariku46.com/competition/", NewsURL: "https://kariku46.com/information/"},
	{Prefecture: "47", PageURL: "https://jaaf-okinawa.com/user.php?CMD=115401800000000", NewsURL: "https://jaaf-okinawa.com/user.php?CMD=115401700000000"},
}

type Source struct {
	Prefecture string
	PageURL    string
	// NewsURL is the prefecture association's news list, read to find guideline links for meets from the annual schedule.
	NewsURL   string
	ExtraURLs []string
	// PDFLinks are patterns for schedule PDFs linked from PageURL. When set, those PDFs are read instead of PageURL.
	PDFLinks []string
}

// ResolveURLs is URLs with PageURL replaced by the PDFs it currently links to.
func (s Source) ResolveURLs(ctx context.Context) ([]string, error) {
	if len(s.PDFLinks) == 0 {
		return s.URLs(), nil
	}
	var urls []string
	for _, pattern := range s.PDFLinks {
		link, err := FindPDFLink(ctx, s.PageURL, pattern)
		if err != nil {
			return nil, err
		}
		urls = append(urls, link)
	}
	return append(urls, s.ExtraURLs...), nil
}

func (s Source) URLs() []string {
	if s.PageURL == "" && len(s.ExtraURLs) == 0 {
		return nil
	}
	if len(s.ExtraURLs) == 0 {
		return []string{s.PageURL}
	}
	urls := append([]string{s.PageURL}, s.ExtraURLs...)
	return urls
}
