package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// 「データ」タブの全体の集計に、5人に満たないグループの回答を混ぜない。混ぜると、全体から公開されている
// グループの分を引くだけで、小さいグループの1人の回答がわかってしまう。
func TestInjuryDataOverallDoesNotRevealSmallGroups(t *testing.T) {
	h := setup(t)
	for i := 0; i < 5; i++ {
		body := joinBodyWith("rehab", map[string]any{"hindsight": []string{"warmup"}})
		if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/jumpers-knee/join", token(t, uuid.New()), body); rec.Code != http.StatusOK {
			t.Fatalf("join %d %s", rec.Code, rec.Body.String())
		}
	}
	lonely := joinBodyWith("rehab", map[string]any{
		"place": "stairs", "activity": "outside", "pre_changes": []string{"played-through-pain"}, "hindsight": []string{"tell-coach"},
	})
	if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/sprain/join", token(t, uuid.New()), lonely); rec.Code != http.StatusOK {
		t.Fatalf("join sprain %d %s", rec.Code, rec.Body.String())
	}

	rec := doJSON(t, h, http.MethodGet, "/v1/injury/data", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("data %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Overall json.RawMessage `json:"overall"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	overall := string(out.Overall)
	for _, code := range []string{`"stairs"`, `"outside"`, `"played-through-pain"`, `"tell-coach"`} {
		if strings.Contains(overall, code) {
			t.Errorf("overall reveals the one-person group's answer %s: %s", code, overall)
		}
	}
	if !strings.Contains(overall, `"tartan"`) || !strings.Contains(overall, `"warmup"`) {
		t.Fatalf("overall should still show the five-person group: %s", overall)
	}
}
