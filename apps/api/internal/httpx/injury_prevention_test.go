package httpx_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type preventionTally struct {
	People int `json:"people"`
	Items  []struct {
		Code   string `json:"code"`
		People int    `json:"people"`
	} `json:"items"`
}

type preventionOut struct {
	Event      string          `json:"event"`
	EventLabel string          `json:"event_label"`
	BodyParts  preventionTally `json:"body_parts"`
	PreChanges preventionTally `json:"pre_changes"`
	Hindsight  preventionTally `json:"hindsight"`
}

func getPrevention(t *testing.T, h http.Handler, event string) preventionOut {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/injury/prevention?event="+event, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("prevention %s: %d %s", event, rec.Code, rec.Body.String())
	}
	var out preventionOut
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func tallyCodes(tally preventionTally) map[string]int {
	out := map[string]int{}
	for _, item := range tally.Items {
		out[item.Code] = item.People
	}
	return out
}

// joinInjury は、プロフィールに種目を入れた新しい人として、part のグループに入る(種目は入るときにプロフィールから写される)。
func joinInjury(t *testing.T, h http.Handler, part, event string, patch map[string]any) {
	t.Helper()
	tok := token(t, uuid.New())
	setProfileEvent(t, h, tok, event)
	if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/"+part+"/join", tok, joinBodyWith("rehab", patch)); rec.Code != http.StatusOK {
		t.Fatalf("join %s: %d %s", part, rec.Code, rec.Body.String())
	}
}

// 種目で絞った予防の集計。5人そろった種目だけ出し、ほかの種目の回答は混ぜない。
// 5人に満たないケガのグループは、種目が合っても数えない(公開しているケガごとの集計と引き比べられないように)。
func TestInjuryPreventionByEvent(t *testing.T) {
	h := setup(t)
	for i := 0; i < 5; i++ {
		joinInjury(t, h, "jumpers-knee", "400m", map[string]any{
			"pre_changes": []string{"more-volume"}, "hindsight": []string{"adjust-volume"},
		})
	}
	// 別の種目の人。ケガのグループは5人を超えるが、400m の集計には入らない。
	joinInjury(t, h, "jumpers-knee", "100m", map[string]any{
		"pre_changes": []string{"new-shoes"}, "hindsight": []string{"warmup"},
	})
	// 1人しかいないグループの 400m の人。数えると1人の回答がわかるので外す。
	joinInjury(t, h, "sprain", "400m", map[string]any{
		"pre_changes": []string{"played-through-pain"}, "hindsight": []string{"tell-coach"},
	})

	got := getPrevention(t, h, "400m")
	if got.Event != "400m" || got.EventLabel == "" {
		t.Fatalf("event %+v", got)
	}
	if got.BodyParts.People != 5 || got.PreChanges.People != 5 || got.Hindsight.People != 5 {
		t.Fatalf("people body=%d pre=%d hindsight=%d", got.BodyParts.People, got.PreChanges.People, got.Hindsight.People)
	}
	parts, pre, hindsight := tallyCodes(got.BodyParts), tallyCodes(got.PreChanges), tallyCodes(got.Hindsight)
	if parts["jumpers-knee"] != 5 || len(parts) != 1 {
		t.Errorf("body parts %v", parts)
	}
	if pre["more-volume"] != 5 || pre["new-shoes"] != 0 || pre["played-through-pain"] != 0 {
		t.Errorf("pre changes %v", pre)
	}
	if hindsight["adjust-volume"] != 5 || hindsight["warmup"] != 0 || hindsight["tell-coach"] != 0 {
		t.Errorf("hindsight %v", hindsight)
	}

	// 100m は1人だけなので、人数だけ返して項目は出さない。
	small := getPrevention(t, h, "100m")
	if small.BodyParts.People != 1 || len(small.BodyParts.Items) != 0 || len(small.PreChanges.Items) != 0 || len(small.Hindsight.Items) != 0 {
		t.Fatalf("one-person event should not show items: %+v", small)
	}
}

func TestInjuryPreventionRejectsUnknownEvent(t *testing.T) {
	h := setup(t)
	for _, event := range []string{"", "sprint", "x"} {
		rec := doJSON(t, h, http.MethodGet, "/v1/injury/prevention?event="+event, "", nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("event %q: %d %s", event, rec.Code, rec.Body.String())
		}
	}
}
