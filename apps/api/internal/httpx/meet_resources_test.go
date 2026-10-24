package httpx

import "testing"

func TestMeetResourcesExternalURLs(t *testing.T) {
	for _, test := range []struct {
		raw   string
		valid bool
	}{
		{"", true}, {" https://example.jp/timetable.pdf ", true}, {"http://example.jp/", true},
		{"javascript:alert(1)", false}, {"data:text/html,test", false}, {"//example.jp", false}, {"https://user:password@example.jp", false},
	} {
		if _, ok := meetExternalURL(test.raw); ok != test.valid {
			t.Errorf("%q valid=%v", test.raw, ok)
		}
	}
	if got := meetOfficialURL("会場: 競技場\n公式サイト: https://example.jp/meet"); got != "https://example.jp/meet" {
		t.Fatal(got)
	}
	if got := meetOfficialURL("公式サイト: javascript:alert(1)"); got != "" {
		t.Fatal(got)
	}
}
