package httpx

import "testing"

func TestNormalizeInjuryCase(t *testing.T) {
	ok := injuryCaseReq{BodyPart: "sprain", Timing: "practice", Situations: []string{"fatigue", "short_warmup"}, Preventions: []string{"rest_early"}}
	if _, fields := normalizeInjuryCase(ok); len(fields) != 0 {
		t.Fatalf("valid input rejected: %v", fields)
	}
	bad := []injuryCaseReq{
		{BodyPart: "x", Timing: "practice", Situations: []string{"fatigue"}, Preventions: []string{"rest_early"}},
		{BodyPart: "sprain", Timing: "x", Situations: []string{"fatigue"}, Preventions: []string{"rest_early"}},
		{BodyPart: "sprain", Timing: "practice", Situations: nil, Preventions: []string{"rest_early"}},
		{BodyPart: "sprain", Timing: "practice", Situations: []string{"fatigue", "fatigue"}, Preventions: []string{"rest_early"}},
		{BodyPart: "sprain", Timing: "practice", Situations: []string{"fatigue"}, Preventions: []string{"a", "b", "c", "d", "e"}},
		{BodyPart: "sprain", Timing: "practice", Situations: []string{"自由記述"}, Preventions: []string{"rest_early"}},
	}
	for i, req := range bad {
		if _, fields := normalizeInjuryCase(req); len(fields) == 0 {
			t.Fatalf("case %d should be rejected", i)
		}
	}
}

func TestBuildInjuryCaseStats(t *testing.T) {
	sprain := newInjuryCaseTally()
	sprain.cases = 6
	sprain.timings["practice"] = 4
	sprain.timings["race"] = 2
	sprain.situations["other"] = 5
	sprain.situations["fatigue"] = 3
	sprain.preventions["rest_early"] = 6
	knee := newInjuryCaseTally()
	knee.cases = 2
	knee.situations["fatigue"] = 2

	got := buildInjuryCaseStats(map[string]*injuryCaseTally{"jumpers-knee": knee, "sprain": sprain})
	if len(got.Parts) != 2 || got.Parts[0].BodyPart != "sprain" {
		t.Fatalf("part order %+v", got.Parts)
	}
	if !got.Parts[0].Ready || got.Parts[1].Ready {
		t.Fatalf("ready flags %+v", got.Parts)
	}
	// 5人に満たない部位は、選択肢ごとの数を返さない。
	if len(got.Parts[1].Situations) != 0 {
		t.Fatalf("small sample leaked: %+v", got.Parts[1])
	}
	// 「その他」は多くても最後。
	s := got.Parts[0].Situations
	if s[0].Code != "fatigue" || s[len(s)-1].Code != "other" || s[0].Percent != 50 {
		t.Fatalf("situations %+v", s)
	}
	if got.Overall.Cases != 8 || !got.Overall.Ready {
		t.Fatalf("overall %+v", got.Overall)
	}
}
