package competitions

import "context"

// Sources are the athletics association homepages to read.
// Other prefectures are added here as their schedule page URL is known.
var Sources = []Source{
	{Prefecture: "01", PageURL: "https://hokkaido-rikkyo.jp/schedule/index.html", PDFLinks: []string{`schedule_hokkaido`}},
	{Prefecture: "02", PageURL: "https://www.jaafaomori.com/schedule/"},
	{Prefecture: "03", PageURL: "https://iwate-rk.com/page-5443/"},
	{
		Prefecture: "04",
		PageURL:    "https://www.jaaf-miyagi.com/schedule/",
		PDFLinks:   []string{`主催・主管・協力大会`, `加盟陸協・協力団体競技会予定`},
	},
	{
		Prefecture: "05",
		PageURL:    "http://jaaf-akita.com/?s=2026&sm=06",
		ExtraURLs: []string{
			"http://jaaf-akita.com/?s=2026&sm=07",
			"http://jaaf-akita.com/?s=2026&sm=08",
			"http://jaaf-akita.com/?s=2026&sm=09",
		},
	},
	{Prefecture: "06", PageURL: "https://jaaf-yamagata.jp/", PDFLinks: []string{`kyouginittei`}},
	{Prefecture: "07", PageURL: "https://gold.jaic.org/fukushima/joho/2026/2026Sche.pdf"},
	{Prefecture: "08", PageURL: "http://ibariku.com/event/2026.html"},
	{Prefecture: "09", PageURL: "https://www.jaaftochigi.jp/event2026/"},
	{Prefecture: "10", PageURL: "http://gold.jaic.org/gunma/competitions.html"},
	{
		Prefecture: "11",
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
	{Prefecture: "12", PageURL: "https://www.jaaf-chiba.jp/", PDFLinks: []string{`_schedule_hp`}},
	{Prefecture: "13", PageURL: "https://toriku.or.jp/competition/"},
	{Prefecture: "14", PageURL: "https://www.kanagawariku.org/kana31com100.html"},
	{Prefecture: "15", PageURL: "https://nrkk.net/official-eventschedule", PDFLinks: []string{`公認競技会`}},
	{Prefecture: "16", PageURL: "https://www.jaaf-toyama.net/schedule/"},
	{Prefecture: "17", PageURL: "https://www.ishiriku.jp/youkou.html"},
	// 年間の主要競技会は「大会・競技場コード」のリンク先 code.pdf に載っている。月ごとのページは冬の月に表が無く失敗する。
	{Prefecture: "18", PageURL: "https://www.fukui-jaaf.com/020_schedule/", PDFLinks: []string{`code\.pdf`}},
	{Prefecture: "19", PageURL: "https://yamanashitf.com/2026nendo-taikai"},
	{Prefecture: "20", PageURL: "https://nagano-rk.com/calendar/2026nrk_calendar-2-2-2-2-2/"},
	{
		Prefecture: "22",
		PageURL:    "https://shizuoka-jaaf.com/srkkyogi.html",
		ExtraURLs:  []string{"https://seiburiku.jp/schedule/"},
	},
	{
		Prefecture: "23",
		PageURL:    "http://aichi-rk.jp/01nittei.htm",
		ExtraURLs: []string{
			"http://aichi-rk.jp/01_01nittei.htm",
			"http://aichi-rk.jp/01_02nittei.htm",
			"http://aichi-rk.jp/01_03nittei.htm",
		},
	},
	{Prefecture: "21", PageURL: "https://gold.jaic.org/gifu/gifuriku/2026/2026%E6%97%A5%E7%A8%8B.pdf"},
	{Prefecture: "24", PageURL: "https://mierk.jp/taikaiyoukou/"},
	{Prefecture: "25", PageURL: "https://srkshiga.com/?page_id=874"},
	{Prefecture: "26", PageURL: "https://www.krk26.jp/", PDFLinks: []string{`競技会日程`}},
	{Prefecture: "27", PageURL: "https://www.oaaa.jp/competition/?y=2026"},
	{Prefecture: "28", PageURL: "http://haaa.jp/taikai.html"},
	{Prefecture: "29", PageURL: "http://www.narariku.com/information.html"},
	{Prefecture: "30", PageURL: "https://wariku.com/athletics-list/"},
	{Prefecture: "31", PageURL: "https://jaaf-tottori.com/competition/"},
	{Prefecture: "32", PageURL: "https://www.shimariku.jp/record", PDFLinks: []string{`陸協行事予定表`}},
	{Prefecture: "33", PageURL: "https://okariku33.com/?page_id=1291"},
	{Prefecture: "34", PageURL: "https://hiroshimatf.org/competitionandschedule/", PDFLinks: []string{`競技会日程`}},
	{Prefecture: "35", PageURL: "https://yaaf.jp/kyougikai-top/schedule-all/"},
	{Prefecture: "36", PageURL: "https://jaaftokushima.com/wp/schedule_table26/"},
	{Prefecture: "37", PageURL: "https://gold.jaic.org/kagawa/2026/2026info.html"},
	{Prefecture: "38", PageURL: "https://www.ehime-rikujyo.jp/01_taikai.html"},
	{Prefecture: "39", PageURL: "https://npo-kochi.sports.coocan.jp/kyogi.files/sheet001.htm"},
	{Prefecture: "40", PageURL: "https://www.fukuriku.com/tournament/"},
	{Prefecture: "41", PageURL: "http://www.sagarikujyo.jp/schedule/schedule_new.html"},
	{Prefecture: "42", PageURL: "https://jaaf-nagasaki.net/schedule/2026ken.htm"},
	{
		Prefecture: "43",
		PageURL:    "http://kumariku.com/schedule/",
		ExtraURLs:  []string{"http://kumariku.com/result/?y=2026"},
	},
	{Prefecture: "44", PageURL: "https://oita-rik.com/schedule"},
	{Prefecture: "45", PageURL: "https://www.miyariku.org/maaf/2026/03/05/2026%E5%B9%B4%E5%BA%A6%E7%AB%B6%E6%8A%80%E4%BC%9A%E6%83%85%E5%A0%B1/"},
	{Prefecture: "46", PageURL: "https://kariku46.com/competition/"},
	{Prefecture: "47", PageURL: "https://jaaf-okinawa.com/user.php?CMD=115401800000000"},
}

type Source struct {
	Prefecture string
	PageURL    string
	ExtraURLs  []string
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
