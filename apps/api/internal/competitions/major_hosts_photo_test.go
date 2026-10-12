package competitions

import "testing"

func TestLeadCityFilePrefersSkylineThenFirstPhoto(t *testing.T) {
	beijing := `
| image_flag = Flag of the People's Republic of China.svg
| image = {{multiple image
| image1        = Skyline of Beijing CBD.jpg
| image2        = Temple of Heaven.jpg
`
	file, ok := leadCityFile(beijing)
	if !ok || file != "Skyline of Beijing CBD.jpg" {
		t.Fatalf("beijing: %q %v", file, ok)
	}

	withSkyline := `
| image1 = Other View.jpg
| image_skyline = Beijing Skyline.jpg
`
	file, ok = leadCityFile(withSkyline)
	if !ok || file != "Beijing Skyline.jpg" {
		t.Fatalf("skyline param: %q %v", file, ok)
	}

	munich := ` |image1                = Stadtbild München.jpg`
	file, ok = leadCityFile(munich)
	if !ok || file != "Stadtbild München.jpg" {
		t.Fatalf("munich: %q %v", file, ok)
	}

	linked := `| image = [[File:Los Angeles with Mount Baldy.jpg|thumb|skyline]]`
	file, ok = leadCityFile(linked)
	if !ok || file != "Los Angeles with Mount Baldy.jpg" {
		t.Fatalf("linked: %q %v", file, ok)
	}
}

func TestLeadCityFileSkipsMontageFlagAndLogo(t *testing.T) {
	text := `
| image_skyline = Beijing_montage_2019.png
| image1 = Flag of Nairobi.svg
| image = 2028 Summer Olympics Logo.svg
`
	if _, ok := leadCityFile(text); ok {
		t.Fatal("montage, flag, and logo should be skipped")
	}
	plain := `| image = Nairobi skyline from Gem Hotel.jpg`
	file, ok := leadCityFile(plain)
	if !ok || file != "Nairobi skyline from Gem Hotel.jpg" {
		t.Fatalf("plain: %q %v", file, ok)
	}
}

func TestEnglishLangTitle(t *testing.T) {
	body := []byte(`{"query":{"pages":[{"title":"北京市","langlinks":[{"lang":"en","title":"Beijing"}]},{"title":"ロサンゼルス","langlinks":[{"lang":"en","title":"Los Angeles (California)"}]}]}}`)
	if got := englishLangTitle(body); got != "Beijing" {
		t.Fatalf("first city: %q", got)
	}
	onlyDisambiguation := []byte(`{"query":{"pages":[{"langlinks":[{"title":"Los Angeles (California)"}]}]}}`)
	if got := englishLangTitle(onlyDisambiguation); got != "Los Angeles" {
		t.Fatalf("disambiguation: %q", got)
	}
	if englishLangTitle([]byte(`{"query":{"pages":[{"title":"なし"}]}}`)) != "" {
		t.Fatal("missing langlink should be empty")
	}
}

func TestChooseCityPhotoPrefersLandscapeSkyline(t *testing.T) {
	images := []commonsImage{
		{Title: "File:Beijing_montage_2019.png", URL: "https://example.test/montage.jpg", Width: 4000, Height: 3000, MIME: "image/png"},
		{Title: "File:Flag of Beijing.svg", URL: "https://example.test/flag.png", Width: 2000, Height: 1200, MIME: "image/svg+xml"},
		{Title: "File:Portrait of a runner.jpg", URL: "https://example.test/portrait.jpg", Width: 1000, Height: 1600, MIME: "image/jpeg"},
		{Title: "File:BBQ with a view.jpg", URL: "https://example.test/bbq.jpg", Width: 4000, Height: 3000, MIME: "image/jpeg"},
		{Title: "File:Beijing Skyline.jpg", URL: "https://example.test/skyline.jpg", Width: 3000, Height: 2000, MIME: "image/jpeg"},
		{Title: "File:Location map Beijing.png", URL: "https://example.test/map.png", Width: 3000, Height: 2000, MIME: "image/png"},
	}
	if got := chooseCityPhoto(images); got != "https://example.test/skyline.jpg" {
		t.Fatalf("chosen: %s", got)
	}
	wide := []commonsImage{{Title: "File:Thin panorama.jpg", URL: "https://example.test/thin.jpg", Width: 5000, Height: 1000, MIME: "image/jpeg"}}
	if chooseCityPhoto(wide) != "" {
		t.Fatal("very wide panorama should be skipped")
	}
}

func TestUsableLeadPhotoRejectsCollageAndLogo(t *testing.T) {
	montage := "https://upload.wikimedia.org/wikipedia/commons/thumb/b/bc/Beijing_montage_2019.png/330px-Beijing_montage_2019.png"
	logo := "https://upload.wikimedia.org/wikipedia/commons/thumb/e/e8/2028_Summer_Olympics_Logo.svg/330px-2028_Summer_Olympics_Logo.svg.png"
	skyline := "https://upload.wikimedia.org/wikipedia/commons/thumb/a/a3/Beijing_Skyline.jpg/1280px-Beijing_Skyline.jpg"
	if usableLeadPhoto(montage) || usableLeadPhoto(logo) {
		t.Fatal("collage and logo should be rejected")
	}
	if !usableLeadPhoto(skyline) {
		t.Fatal("skyline thumbnail should be kept")
	}
}

func TestParseCommonsImagesDropsTrackingQuery(t *testing.T) {
	body := []byte(`{"query":{"pages":[{"title":"File:Beijing Skyline.jpg","imageinfo":[{"thumburl":"https://upload.wikimedia.org/wikipedia/commons/thumb/a/a3/Beijing_Skyline.jpg/1280px-Beijing_Skyline.jpg?utm_source=commons","width":3000,"height":2000,"mime":"image/jpeg"}]}]}}`)
	images := parseCommonsImages(body)
	want := "https://upload.wikimedia.org/wikipedia/commons/thumb/a/a3/Beijing_Skyline.jpg/1280px-Beijing_Skyline.jpg"
	if len(images) != 1 || images[0].URL != want {
		t.Fatalf("url: %+v", images)
	}
}

func TestParseCommonsImages(t *testing.T) {
	body := []byte(`{"query":{"pages":[
		{"title":"File:Missing.jpg","missing":true},
		{"title":"File:Beijing Skyline.jpg","imageinfo":[{"thumburl":"https://example.test/beijing.jpg","width":3000,"height":2000,"mime":"image/jpeg"}]}
	]}}`)
	images := parseCommonsImages(body)
	if len(images) != 1 || images[0].URL != "https://example.test/beijing.jpg" || images[0].Width != 3000 {
		t.Fatalf("images: %+v", images)
	}
}
