package competitions

// Sources are the athletics association homepages to read.
// Other prefectures are added here as their schedule page URL is known.
var Sources = []Source{
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
	{Prefecture: "13", PageURL: "https://toriku.or.jp/competition/"},
	{Prefecture: "14", PageURL: "https://www.kanagawariku.org/kana31com100.html"},
	{Prefecture: "16", PageURL: "https://www.jaaf-toyama.net/schedule/"},
	{Prefecture: "17", PageURL: "https://www.ishiriku.jp/youkou.html"},
	{
		Prefecture: "18",
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
	{Prefecture: "24", PageURL: "https://mierk.jp/taikaiyoukou/"},
	{Prefecture: "25", PageURL: "https://srkshiga.com/?page_id=874"},
	{Prefecture: "27", PageURL: "https://www.oaaa.jp/competition/?y=2026"},
	{Prefecture: "28", PageURL: "http://haaa.jp/taikai.html"},
	{Prefecture: "29", PageURL: "http://www.narariku.com/information.html"},
	{Prefecture: "30", PageURL: "https://wariku.com/athletics-list/"},
	{Prefecture: "31", PageURL: "https://jaaf-tottori.com/competition/"},
	{Prefecture: "33", PageURL: "https://okariku33.com/?page_id=1291"},
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
