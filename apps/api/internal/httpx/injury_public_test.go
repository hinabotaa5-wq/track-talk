package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type publicGroupResp struct {
	Members   int64 `json:"members"`
	MinPeople int   `json:"min_people"`
	Recovery  []struct {
		Severity string `json:"severity"`
		Stages   []struct {
			Stage    string `json:"stage"`
			People   int    `json:"people"`
			LowDays  *int   `json:"low_days"`
			HighDays *int   `json:"high_days"`
		} `json:"stages"`
	} `json:"recovery"`
	Places struct {
		People int `json:"people"`
		Items  []struct {
			Code   string `json:"code"`
			People int    `json:"people"`
		} `json:"items"`
	} `json:"places"`
	Actions struct {
		People int `json:"people"`
		Items  []struct {
			Code   string `json:"code"`
			People int    `json:"people"`
		} `json:"items"`
	} `json:"actions"`
	Notes []map[string]any `json:"notes"`
}

func getPublicGroup(t *testing.T, h http.Handler, part string) (publicGroupResp, string) {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/injury/groups/"+part+"/public", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("public group %d %s", rec.Code, rec.Body.String())
	}
	var out publicGroupResp
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out, rec.Body.String()
}

func TestInjuryPublicGroupShowsAggregatesFromFivePeople(t *testing.T) {
	h := setup(t)
	if rec := doJSON(t, h, http.MethodGet, "/v1/injury/groups/knee/public", "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown group %d", rec.Code)
	}

	members := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		tok := token(t, uuid.New())
		if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/shin-splints/join", tok, joinBody("rehab")); rec.Code != http.StatusOK {
			t.Fatalf("join %d %s", rec.Code, rec.Body.String())
		}
		members = append(members, tok)
		if i == 3 {
			// 4人では、誰のことかわかってしまうので集計を出さない。
			g, _ := getPublicGroup(t, h, "shin-splints")
			if g.Members != 4 || g.Places.People != 4 || len(g.Places.Items) != 0 {
				t.Fatalf("four people %+v", g)
			}
		}
	}

	for i, tok := range members {
		body := map[string]any{"stage": "jogging", "actions": []string{"stretch", "icing"}}
		if i == 0 {
			body = map[string]any{"stage": "jogging", "actions": []string{"stretch"}, "note": "焦らずに週3から", "note_public": true}
		}
		if i == 1 {
			body = map[string]any{"stage": "jogging", "actions": []string{"stretch"}, "note": "これは仲間だけに", "note_public": false}
		}
		if rec := doJSON(t, h, http.MethodPatch, "/v1/injury/groups/shin-splints/me", tok, body); rec.Code != http.StatusOK {
			t.Fatalf("change %d %s", rec.Code, rec.Body.String())
		}
	}

	g, raw := getPublicGroup(t, h, "shin-splints")
	if g.Members != 5 || g.MinPeople != 5 {
		t.Fatalf("group %+v", g)
	}
	if len(g.Places.Items) != 1 || g.Places.Items[0].Code != "tartan" || g.Places.Items[0].People != 5 {
		t.Fatalf("places %+v", g.Places)
	}
	if g.Actions.People != 5 || g.Actions.Items[0].Code != "stretch" || g.Actions.Items[0].People != 5 || g.Actions.Items[1].People != 3 {
		t.Fatalf("actions %+v", g.Actions)
	}
	var jog *struct {
		Stage    string `json:"stage"`
		People   int    `json:"people"`
		LowDays  *int   `json:"low_days"`
		HighDays *int   `json:"high_days"`
	}
	for _, r := range g.Recovery {
		if r.Severity == "moderate" {
			jog = &r.Stages[0]
		}
	}
	if jog == nil || jog.Stage != "jogging" || jog.People != 5 || jog.LowDays == nil || jog.HighDays == nil || *jog.LowDays > *jog.HighDays {
		t.Fatalf("recovery %+v", g.Recovery)
	}

	// 公開してよいと選んだ一言だけが出て、ニックネームは出ない。
	if len(g.Notes) != 1 || g.Notes[0]["note"] != "焦らずに週3から" || g.Notes[0]["stage_label"] != "ジョグ再開" || g.Notes[0]["event_label"] != "短距離" {
		t.Fatalf("notes %+v", g.Notes)
	}
	if strings.Contains(raw, "仲間だけに") || strings.Contains(raw, "nickname") || strings.Contains(raw, "user_id") {
		t.Fatalf("public page leaks: %s", raw)
	}
}
