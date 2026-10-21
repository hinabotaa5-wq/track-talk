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
		setProfileEvent(t, h, tok, "100m")
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
	if len(g.Notes) != 1 || g.Notes[0]["note"] != "焦らずに週3から" || g.Notes[0]["stage_label"] != "ジョグ再開" || g.Notes[0]["event_label"] != "100m" {
		t.Fatalf("notes %+v", g.Notes)
	}
	if strings.Contains(raw, "仲間だけに") || strings.Contains(raw, "nickname") || strings.Contains(raw, "user_id") {
		t.Fatalf("public page leaks: %s", raw)
	}
}

func TestInjuryPublicNoteCanBeReportedAndHidden(t *testing.T) {
	h := setup(t)
	writer, reader, adminID := token(t, uuid.New()), token(t, uuid.New()), uuid.New()
	admin := token(t, adminID)
	doJSON(t, h, http.MethodGet, "/v1/me", admin, nil)
	setAdmin(t, adminID)

	doJSON(t, h, http.MethodPost, "/v1/injury/groups/elbow/join", writer, joinBody("rehab"))
	doJSON(t, h, http.MethodPatch, "/v1/injury/groups/elbow/me", writer, map[string]any{
		"stage": "jogging", "actions": []string{"icing"}, "note": "不適切な一言", "note_public": true,
	})
	g, _ := getPublicGroup(t, h, "elbow")
	if len(g.Notes) != 1 {
		t.Fatalf("notes %+v", g.Notes)
	}
	id, _ := g.Notes[0]["id"].(string)

	report := func(target string) int {
		return doJSON(t, h, http.MethodPost, "/v1/reports", reader, map[string]any{
			"target_type": "injury_note", "target_id": target, "reason": "harassment", "detail": "",
		}).Code
	}
	if code := report(uuid.NewString()); code != http.StatusNotFound {
		t.Fatalf("unknown note %d", code)
	}
	if code := report(id); code != http.StatusCreated {
		t.Fatalf("report %d", code)
	}
	list := doJSON(t, h, http.MethodGet, "/v1/admin/reports", admin, nil)
	if !strings.Contains(list.Body.String(), "不適切な一言") || !strings.Contains(list.Body.String(), `"injury_part":"elbow"`) {
		t.Fatalf("admin list %s", list.Body.String())
	}

	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/injury-notes/"+id+"/hide", reader, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin hide %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/injury-notes/"+id+"/hide", admin, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("hide %d %s", rec.Code, rec.Body.String())
	}
	if g, _ := getPublicGroup(t, h, "elbow"); len(g.Notes) != 0 {
		t.Fatalf("hidden note still public %+v", g.Notes)
	}
	if code := report(id); code != http.StatusNotFound {
		t.Fatalf("report hidden note %d", code)
	}
	if !strings.Contains(doJSON(t, h, http.MethodGet, "/v1/admin/reports", admin, nil).Body.String(), `"target_hidden":true`) {
		t.Fatal("admin list should show the note as hidden")
	}
}
