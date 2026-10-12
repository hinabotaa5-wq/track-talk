package competitions

// Sources are the athletics association homepages to read.
// Other prefectures are added here as their schedule page URL is known.
var Sources = []Source{
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
	{Prefecture: "43", PageURL: "http://kumariku.com/schedule/"},
	{Prefecture: "44", PageURL: "https://oita-rik.com/schedule"},
	{Prefecture: "45", PageURL: "https://www.miyariku.org/maaf/2026/03/05/2026%E5%B9%B4%E5%BA%A6%E7%AB%B6%E6%8A%80%E4%BC%9A%E6%83%85%E5%A0%B1/"},
	{Prefecture: "46", PageURL: "https://kariku46.com/competition/"},
	{Prefecture: "47", PageURL: "https://jaaf-okinawa.com/user.php?CMD=115401800000000"},
}

type Source struct {
	Prefecture string
	PageURL    string
}
