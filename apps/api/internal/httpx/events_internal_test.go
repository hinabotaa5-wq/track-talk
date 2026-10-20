package httpx

import "testing"

func TestEventsAreSharedAcrossTheApp(t *testing.T) {
	// コーチの担当種目・練習相手募集・利用者の種目などは、すべて同じ一覧。
	if len(coachEvents) != len(appEvents) {
		t.Fatalf("coach events %v", coachEvents)
	}
	for _, code := range []string{"100m", "race-walk", "relay", "heptathlon", "decathlon"} {
		if !validEvent(code) || !validPracticeEvent(code) {
			t.Fatalf("%s is not selectable", code)
		}
	}
	// 以前の区分や「その他」は、もう選べない。
	for _, code := range []string{"other", "sprint", "4x100", "4x400"} {
		if validEvent(code) || validPracticeEvent(code) {
			t.Fatalf("%s is still selectable", code)
		}
	}
	// 選べなくなった種目も、保存してある値は表示できる。
	if eventLabel("heptathlon") != "七種競技" || eventLabel("4x100") != "4×100mR" || eventLabel("sprint") != "短距離" || eventLabel("relay") != "リレー" {
		t.Fatal("labels")
	}
	for _, code := range []string{"race-walk", "relay"} {
		if spec, ok := raceEvents[code]; !ok || spec.kind != raceMarkTime {
			t.Fatalf("%s records", code)
		}
	}
}
