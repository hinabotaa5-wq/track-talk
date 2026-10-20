package httpx

import "testing"

func TestEventsAreSharedAcrossTheApp(t *testing.T) {
	// リレーは、コーチの担当種目と練習相手募集だけ。
	if validEvent(relayEvent) || !validPracticeEvent(relayEvent) {
		t.Fatalf("relay: app %v practice %v", validEvent(relayEvent), validPracticeEvent(relayEvent))
	}
	found := false
	for _, code := range coachEvents {
		if code == relayEvent {
			found = true
		}
	}
	if !found || len(coachEvents) != len(appEvents)+1 {
		t.Fatalf("coach events %v", coachEvents)
	}
	// 以前の区分や「その他」は、もう選べない。
	for _, code := range []string{"other", "heptathlon", "decathlon", "4x100", "4x400", "sprint"} {
		if validEvent(code) || validPracticeEvent(code) {
			t.Fatalf("%s is still selectable", code)
		}
	}
	// 選べなくなった種目も、保存してある値は表示できる。
	if eventLabel("heptathlon") != "七種競技" || eventLabel("sprint") != "短距離" || eventLabel("race-walk") != "競歩" {
		t.Fatal("labels")
	}
	if spec, ok := raceEvents["race-walk"]; !ok || spec.kind != raceMarkTime {
		t.Fatal("race-walk records")
	}
}
