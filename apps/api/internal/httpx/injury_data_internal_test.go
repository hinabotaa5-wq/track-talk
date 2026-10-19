package httpx

import "testing"

func TestInjuryDataOverallHidesSmallSamples(t *testing.T) {
	// 5人に満たないうちは、全体の集計も項目を出さない。
	got := buildPublicInjuryGroup("", nil, nil, nil, nil)
	if got.Members != 0 || len(got.Activities.Items) != 0 || len(got.Actions.Items) != 0 {
		t.Fatalf("unexpected %+v", got)
	}
}
