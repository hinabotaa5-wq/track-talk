package competitions

// Sources are the athletics association homepages to read.
// Other prefectures are added here as their schedule page URL is known.
var Sources = []Source{
	{Prefecture: "46", PageURL: "https://kariku46.com/competition/"},
	{Prefecture: "47", PageURL: "https://jaaf-okinawa.com/user.php?CMD=115401800000000"},
}

type Source struct {
	Prefecture string
	PageURL    string
}
