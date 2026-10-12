package httpx

import "testing"

func TestPrefectureKeywordMatchesNames(t *testing.T) {
	tokyo := prefectureCodesForKeyword("東京")
	if len(tokyo) != 1 || tokyo[0] != "13" {
		t.Fatalf("tokyo %#v", tokyo)
	}
	hokkaido := prefectureCodesForKeyword("北海道")
	if len(hokkaido) != 1 || hokkaido[0] != "01" {
		t.Fatalf("hokkaido %#v", hokkaido)
	}
	if len(prefectureCodesForKeyword("")) != 0 {
		t.Fatal("empty keyword should not select prefectures")
	}
}

func TestNormalizeRecruitment(t *testing.T) {
	in, fields := normalizeRecruitment(postReq{
		Prefectures: []string{"13", "13", "14"},
		Venue:       " 駒沢競技場 ",
		Schedule:    "毎週火曜 19:00-21:00",
		Links:       []string{" https://example.com/team ", ""},
	})
	if len(fields) != 0 {
		t.Fatalf("fields %#v", fields)
	}
	if in.Venue != "駒沢競技場" || len(in.Prefectures) != 2 || in.Prefectures[0] != "13" || len(in.Links) != 1 {
		t.Fatalf("%#v", in)
	}

	_, fields = normalizeRecruitment(postReq{
		Prefectures: []string{},
		Schedule:    "",
		Links:       []string{"http://example.com"},
	})
	if fields["prefectures"] == "" || fields["schedule"] == "" || fields["links"] == "" {
		t.Fatalf("fields %#v", fields)
	}

	_, fields = normalizeRecruitment(postReq{
		Prefectures: []string{"13"},
		Schedule:    "毎週",
		Links:       []string{"https://a.example", "https://b.example", "https://c.example", "https://d.example", "https://e.example"},
	})
	if fields["links"] != "URLは4件までです" {
		t.Fatalf("links %#v", fields)
	}

	_, fields = normalizeRecruitment(postReq{
		Prefectures:    []string{"13"},
		Schedule:       "毎週",
		ImagePublicIDs: []string{"a", "b", "c", "d"},
	})
	if fields["image_public_ids"] != "写真は3枚までです" {
		t.Fatalf("fields %#v", fields)
	}
}
