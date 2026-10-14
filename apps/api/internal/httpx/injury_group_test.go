package httpx_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func joinBody(stage string) map[string]any {
	return map[string]any{"injured_on": "2026-09-20", "place": "練習中のトラック", "situation": "着地で膝をひねった", "stage": stage}
}

func TestInjuryGroupRequiresJoining(t *testing.T) {
	h := setup(t)
	tok := token(t, uuid.New())

	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/v1/injury/groups/knee"},
		{http.MethodGet, "/v1/injury/groups/knee/posts"},
		{http.MethodPatch, "/v1/injury/groups/knee/me"},
		{http.MethodPost, "/v1/injury/groups/knee/posts"},
	} {
		rec := doJSON(t, h, c.method, c.path, tok, map[string]any{"stage": "rehab", "kind": "injury", "body": "x", "body_part": "knee"})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s before joining: %d %s", c.method, c.path, rec.Code, rec.Body.String())
		}
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/injury/groups/head", tok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown part %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/knee/join", "", joinBody("rehab")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("join without login %d", rec.Code)
	}
}

func TestInjuryGroupJoinValidation(t *testing.T) {
	h := setup(t)
	tok := token(t, uuid.New())
	for name, body := range map[string]map[string]any{
		"no date":      {"place": "a", "situation": "b", "stage": "rehab"},
		"future date":  {"injured_on": "2999-01-01", "place": "a", "situation": "b", "stage": "rehab"},
		"no place":     {"injured_on": "2026-09-20", "place": " ", "situation": "b", "stage": "rehab"},
		"no situation": {"injured_on": "2026-09-20", "place": "a", "situation": "", "stage": "rehab"},
		"bad stage":    {"injured_on": "2026-09-20", "place": "a", "situation": "b", "stage": "sprint"},
	} {
		if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/knee/join", tok, body); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
}

type groupResp struct {
	Members int64 `json:"members"`
	Stages  []struct {
		Stage   string   `json:"stage"`
		Members int64    `json:"members"`
		Faces   []string `json:"faces"`
	} `json:"stages"`
	Me struct {
		Stage string `json:"stage"`
	} `json:"me"`
}

func getGroup(t *testing.T, h http.Handler, part, tok string) groupResp {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/injury/groups/"+part, tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get group %d %s", rec.Code, rec.Body.String())
	}
	var out groupResp
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestInjuryGroupStagesMoveForwardAndBack(t *testing.T) {
	h := setup(t)
	a, b := token(t, uuid.New()), token(t, uuid.New())

	if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/knee/join", a, joinBody("resting")); rec.Code != http.StatusOK {
		t.Fatalf("join a %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/knee/join", b, joinBody("jogging")); rec.Code != http.StatusOK {
		t.Fatalf("join b %d %s", rec.Code, rec.Body.String())
	}

	g := getGroup(t, h, "knee", a)
	if g.Members != 2 || g.Me.Stage != "resting" || len(g.Stages) != 5 {
		t.Fatalf("group %+v", g)
	}
	if g.Stages[0].Stage != "resting" || g.Stages[0].Members != 1 || g.Stages[2].Members != 1 {
		t.Fatalf("stage counts %+v", g.Stages)
	}

	// 段階を飛ばして進めることも、戻すこともできる。
	for _, stage := range []string{"race", "rehab"} {
		if rec := doJSON(t, h, http.MethodPatch, "/v1/injury/groups/knee/me", a, map[string]any{"stage": stage}); rec.Code != http.StatusOK {
			t.Fatalf("move to %s: %d %s", stage, rec.Code, rec.Body.String())
		}
		if got := getGroup(t, h, "knee", a).Me.Stage; got != stage {
			t.Fatalf("stage after move: %s want %s", got, stage)
		}
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/injury/groups/knee/me", a, map[string]any{"stage": "sprint"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad stage %d", rec.Code)
	}
	// 他の部位のグループには入っていない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/injury/groups/ankle", a, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("other part %d", rec.Code)
	}

	list := doJSON(t, h, http.MethodGet, "/v1/injury/groups", a, nil)
	var groups struct {
		Groups []struct {
			BodyPart string `json:"body_part"`
			Members  int64  `json:"members"`
			Me       *struct {
				Stage string `json:"stage"`
			} `json:"me"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &groups); err != nil {
		t.Fatal(err)
	}
	for _, item := range groups.Groups {
		switch item.BodyPart {
		case "knee":
			if item.Members != 2 || item.Me == nil || item.Me.Stage != "rehab" {
				t.Fatalf("knee summary %+v", item)
			}
		default:
			if item.Me != nil {
				t.Fatalf("%s should not show me", item.BodyPart)
			}
		}
	}
}

func TestInjuryGroupPostsStayInTheirStage(t *testing.T) {
	h := setup(t)
	a, b := token(t, uuid.New()), token(t, uuid.New())
	doJSON(t, h, http.MethodPost, "/v1/injury/groups/knee/join", a, joinBody("resting"))
	doJSON(t, h, http.MethodPost, "/v1/injury/groups/knee/join", b, joinBody("jogging"))

	post := func(tok, part, body string) int {
		return doJSON(t, h, http.MethodPost, "/v1/injury/groups/knee/posts", tok, map[string]any{
			"kind": "injury", "body": body, "body_part": part,
		}).Code
	}
	if code := post(a, "ankle", "部位違い"); code != http.StatusUnprocessableEntity {
		t.Fatalf("wrong part %d", code)
	}
	if code := post(a, "knee", "階段がつらい"); code != http.StatusCreated {
		t.Fatalf("post a %d", code)
	}
	if code := post(b, "knee", "5分ジョグできた"); code != http.StatusCreated {
		t.Fatalf("post b %d", code)
	}

	// 全段階の投稿を読める。段階で絞ると、投稿した時点の段階だけが出る。
	if all := listInjury(t, h, "/v1/injury/groups/knee/posts", a); all.Meta.Total != 2 {
		t.Fatalf("group feed %+v", all)
	}
	if jog := listInjury(t, h, "/v1/injury/groups/knee/posts?stage=jogging", a); jog.Meta.Total != 1 {
		t.Fatalf("jogging feed %+v", jog)
	}

	// 段階を戻しても、書いた投稿は書いた時点の段階のまま。
	doJSON(t, h, http.MethodPatch, "/v1/injury/groups/knee/me", b, map[string]any{"stage": "rehab"})
	if jog := listInjury(t, h, "/v1/injury/groups/knee/posts?stage=jogging", a); jog.Meta.Total != 1 {
		t.Fatalf("post moved with its writer %+v", jog)
	}
	if rehab := listInjury(t, h, "/v1/injury/groups/knee/posts?stage=rehab", a); rehab.Meta.Total != 0 {
		t.Fatalf("rehab feed %+v", rehab)
	}
	if code := post(b, "knee", "戻ったので今はリハビリ"); code != http.StatusCreated {
		t.Fatalf("post after moving back %d", code)
	}
	if rehab := listInjury(t, h, "/v1/injury/groups/knee/posts?stage=rehab", a); rehab.Meta.Total != 1 {
		t.Fatalf("rehab feed after post %+v", rehab)
	}
}

func TestInjuryGroupCanBeLeftAndJoinedAgain(t *testing.T) {
	h := setup(t)
	a, b := token(t, uuid.New()), token(t, uuid.New())
	doJSON(t, h, http.MethodPost, "/v1/injury/groups/knee/join", a, joinBody("rehab"))
	doJSON(t, h, http.MethodPost, "/v1/injury/groups/knee/join", b, joinBody("jogging"))
	rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/knee/posts", a, map[string]any{"kind": "injury", "body": "抜ける前の投稿", "body_part": "knee"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("post %d %s", rec.Code, rec.Body.String())
	}
	var post struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &post); err != nil {
		t.Fatal(err)
	}

	if r := doJSON(t, h, http.MethodDelete, "/v1/injury/groups/knee/me", "", nil); r.Code != http.StatusUnauthorized {
		t.Fatalf("leave without login %d", r.Code)
	}
	if r := doJSON(t, h, http.MethodDelete, "/v1/injury/groups/ankle/me", a, nil); r.Code != http.StatusForbidden {
		t.Fatalf("leave a group never joined %d", r.Code)
	}
	if r := doJSON(t, h, http.MethodDelete, "/v1/injury/groups/knee/me", a, nil); r.Code != http.StatusNoContent {
		t.Fatalf("leave %d %s", r.Code, r.Body.String())
	}

	// 抜けるとグループの中は読めなくなり、人数も減る。自分の投稿は、自分なら今までどおり読める。
	if r := doJSON(t, h, http.MethodGet, "/v1/injury/groups/knee", a, nil); r.Code != http.StatusForbidden {
		t.Fatalf("group still readable after leaving %d", r.Code)
	}
	if g := getGroup(t, h, "knee", b); g.Members != 1 {
		t.Fatalf("members after leaving %+v", g)
	}
	if r := doJSON(t, h, http.MethodGet, "/v1/posts/"+post.ID, a, nil); r.Code != http.StatusOK {
		t.Fatalf("own post after leaving %d", r.Code)
	}
	// 書いた投稿は、グループに残る。
	if feed := listInjury(t, h, "/v1/injury/groups/knee/posts", b); feed.Meta.Total != 1 {
		t.Fatalf("post should stay in the group %+v", feed)
	}

	// もう一度入れる。
	if r := doJSON(t, h, http.MethodPost, "/v1/injury/groups/knee/join", a, joinBody("resting")); r.Code != http.StatusOK {
		t.Fatalf("join again %d", r.Code)
	}
	if g := getGroup(t, h, "knee", a); g.Members != 2 || g.Me.Stage != "resting" {
		t.Fatalf("after joining again %+v", g)
	}
}
