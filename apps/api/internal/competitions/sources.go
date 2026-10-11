package competitions

// Sources are the athletics association homepages to read.
// Other prefectures are added here as their schedule page URL is known.
var Sources = []Source{
	{Prefecture: "40", PageURL: "https://www.fukuriku.com/tournament/"},
	{Prefecture: "41", PageURL: "http://www.sagarikujyo.jp/schedule/schedule_new.html"},
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
