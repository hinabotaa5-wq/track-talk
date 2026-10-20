package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func joinBody(stage string) map[string]any {
	return map[string]any{
		"severity": "moderate", "saw_doctor": true, "injured_on": "2026-09-20", "place": "tartan",
		"activity": "sprint", "pre_changes": []string{"more-volume", "new-shoes"}, "stage": stage,
	}
}

func stageBody(stage string) map[string]any {
	return map[string]any{"stage": stage, "actions": []string{"stretch"}}
}

func joinBodyWith(stage string, patch map[string]any) map[string]any {
	body := joinBody(stage)
	for k, v := range patch {
		if v == nil {
			delete(body, k)
		} else {
			body[k] = v
		}
	}
	return body
}

func TestInjuryGroupRequiresJoining(t *testing.T) {
	h := setup(t)
	tok := token(t, uuid.New())

	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/v1/injury/groups/jumpers-knee"},
		{http.MethodGet, "/v1/injury/groups/jumpers-knee/posts"},
		{http.MethodPatch, "/v1/injury/groups/jumpers-knee/me"},
		{http.MethodPost, "/v1/injury/groups/jumpers-knee/posts"},
	} {
		rec := doJSON(t, h, c.method, c.path, tok, map[string]any{"stage": "rehab", "kind": "injury", "body": "x", "body_part": "jumpers-knee"})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s before joining: %d %s", c.method, c.path, rec.Code, rec.Body.String())
		}
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/injury/groups/head", tok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown part %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/jumpers-knee/join", "", joinBody("rehab")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("join without login %d", rec.Code)
	}
}

func TestInjuryGroupJoinValidation(t *testing.T) {
	h := setup(t)
	tok := token(t, uuid.New())
	for name, body := range map[string]map[string]any{
		"no severity":      joinBodyWith("rehab", map[string]any{"severity": nil}),
		"bad severity":     joinBodyWith("rehab", map[string]any{"severity": "light"}),
		"no doctor answer": joinBodyWith("rehab", map[string]any{"saw_doctor": nil}),
		"no date":          joinBodyWith("rehab", map[string]any{"injured_on": nil}),
		"month only":       joinBodyWith("rehab", map[string]any{"injured_on": "2026-09"}),
		"future date":      joinBodyWith("rehab", map[string]any{"injured_on": "2999-01-01"}),
		"too old":          joinBodyWith("rehab", map[string]any{"injured_on": "2000-01-01"}),
		"bad place":        joinBodyWith("rehab", map[string]any{"place": "練習中のトラック"}),
		"bad activity":     joinBodyWith("rehab", map[string]any{"activity": "run"}),
		"no pre changes":   joinBodyWith("rehab", map[string]any{"pre_changes": []string{}}),
		"none with others": joinBodyWith("rehab", map[string]any{"pre_changes": []string{"none", "fatigue"}}),
		"bad pre change":   joinBodyWith("rehab", map[string]any{"pre_changes": []string{"rain"}}),
		"bad stage":        joinBodyWith("sprint", nil),
	} {
		if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/jumpers-knee/join", tok, body); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
}

// 種目は登録のときに選ぶので、ケガのグループに入る前に、プロフィールに種目を入れておく。
func setProfileEvent(t *testing.T, h http.Handler, tok, event string) {
	t.Helper()
	body := map[string]any{"nickname": "テスト", "nickname_public": false, "event": event}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/me", tok, body); rec.Code != http.StatusOK {
		t.Fatalf("set event %d %s", rec.Code, rec.Body.String())
	}
}

func TestInjuryGroupJoinAnswers(t *testing.T) {
	h := setup(t)
	tok := token(t, uuid.New())
	setProfileEvent(t, h, tok, "100m")

	// 「その他」では、グループ名がケガを表さないので、ケガの名前を書いてもらう。
	if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/other/join", tok, joinBody("resting")); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("other without diagnosis: %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/other/join", tok, joinBodyWith("resting", map[string]any{"diagnosis": " 脛骨の疲労骨折 "}))
	if rec.Code != http.StatusOK {
		t.Fatalf("join other: %d %s", rec.Code, rec.Body.String())
	}
	var me struct {
		InjuredOn     string   `json:"injured_on"`
		Diagnosis     string   `json:"diagnosis"`
		SeverityLabel string   `json:"severity_label"`
		SawDoctor     *bool    `json:"saw_doctor"`
		PlaceLabel    string   `json:"place_label"`
		ActivityLabel string   `json:"activity_label"`
		PreChanges    []string `json:"pre_changes"`
		EventLabel    string   `json:"event_label"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if me.InjuredOn != "2026-09-20" || me.Diagnosis != "脛骨の疲労骨折" || me.SeverityLabel != "中くらい(練習を数週間休む)" ||
		me.SawDoctor == nil || !*me.SawDoctor || me.PlaceLabel != "タータン" || me.ActivityLabel != "中間から後半を全力疾走" ||
		len(me.PreChanges) != 2 || me.EventLabel != "100m" {
		t.Fatalf("answers %+v", me)
	}

	// ケガの名前を聞くのは「その他」だけ。ほかのグループでは送られても保存しない。
	rec = doJSON(t, h, http.MethodPost, "/v1/injury/groups/sprain/join", tok, joinBodyWith("rehab", map[string]any{"diagnosis": "捻挫"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("join sprain: %d %s", rec.Code, rec.Body.String())
	}
	var sprain struct {
		Diagnosis string `json:"diagnosis"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &sprain); err != nil {
		t.Fatal(err)
	}
	if sprain.Diagnosis != "" {
		t.Fatalf("diagnosis outside other: %q", sprain.Diagnosis)
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

	if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/jumpers-knee/join", a, joinBody("resting")); rec.Code != http.StatusOK {
		t.Fatalf("join a %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/jumpers-knee/join", b, joinBody("jogging")); rec.Code != http.StatusOK {
		t.Fatalf("join b %d %s", rec.Code, rec.Body.String())
	}

	g := getGroup(t, h, "jumpers-knee", a)
	if g.Members != 2 || g.Me.Stage != "resting" || len(g.Stages) != 6 {
		t.Fatalf("group %+v", g)
	}
	if g.Stages[0].Stage != "resting" || g.Stages[0].Members != 1 || g.Stages[2].Members != 1 {
		t.Fatalf("stage counts %+v", g.Stages)
	}

	// 段階を飛ばして進めることも、戻すこともできる。
	for _, stage := range []string{"race", "rehab"} {
		if rec := doJSON(t, h, http.MethodPatch, "/v1/injury/groups/jumpers-knee/me", a, stageBody(stage)); rec.Code != http.StatusOK {
			t.Fatalf("move to %s: %d %s", stage, rec.Code, rec.Body.String())
		}
		if got := getGroup(t, h, "jumpers-knee", a).Me.Stage; got != stage {
			t.Fatalf("stage after move: %s want %s", got, stage)
		}
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/injury/groups/jumpers-knee/me", a, stageBody("sprint")); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad stage %d", rec.Code)
	}
	// 他の部位のグループには入っていない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/injury/groups/sprain", a, nil); rec.Code != http.StatusForbidden {
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
		case "jumpers-knee":
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
	doJSON(t, h, http.MethodPost, "/v1/injury/groups/jumpers-knee/join", a, joinBody("resting"))
	doJSON(t, h, http.MethodPost, "/v1/injury/groups/jumpers-knee/join", b, joinBody("jogging"))

	post := func(tok, part, title, body string) int {
		return doJSON(t, h, http.MethodPost, "/v1/injury/groups/jumpers-knee/posts", tok, map[string]any{
			"kind": "injury", "title": title, "body": body, "body_part": part,
		}).Code
	}
	if code := post(a, "sprain", "部位違い", "部位違い"); code != http.StatusUnprocessableEntity {
		t.Fatalf("wrong part %d", code)
	}
	if code := post(a, "jumpers-knee", "階段の状態", "階段がつらい"); code != http.StatusCreated {
		t.Fatalf("post a %d", code)
	}
	if code := post(b, "jumpers-knee", "ジョギング", "5分ジョグできた"); code != http.StatusCreated {
		t.Fatalf("post b %d", code)
	}

	// 全段階の投稿を読める。段階で絞ると、投稿した時点の段階だけが出る。
	if all := listInjury(t, h, "/v1/injury/groups/jumpers-knee/posts", a); all.Meta.Total != 2 {
		t.Fatalf("group feed %+v", all)
	}
	if jog := listInjury(t, h, "/v1/injury/groups/jumpers-knee/posts?stage=jogging", a); jog.Meta.Total != 1 {
		t.Fatalf("jogging feed %+v", jog)
	}

	// 段階を戻しても、書いた投稿は書いた時点の段階のまま。
	doJSON(t, h, http.MethodPatch, "/v1/injury/groups/jumpers-knee/me", b, stageBody("rehab"))
	if jog := listInjury(t, h, "/v1/injury/groups/jumpers-knee/posts?stage=jogging", a); jog.Meta.Total != 1 {
		t.Fatalf("post moved with its writer %+v", jog)
	}
	if rehab := listInjury(t, h, "/v1/injury/groups/jumpers-knee/posts?stage=rehab", a); rehab.Meta.Total != 0 {
		t.Fatalf("rehab feed %+v", rehab)
	}
	if code := post(b, "jumpers-knee", "リハビリ復帰", "戻ったので今はリハビリ"); code != http.StatusCreated {
		t.Fatalf("post after moving back %d", code)
	}
	if rehab := listInjury(t, h, "/v1/injury/groups/jumpers-knee/posts?stage=rehab", a); rehab.Meta.Total != 1 {
		t.Fatalf("rehab feed after post %+v", rehab)
	}
}

func TestInjuryGroupCanBeLeftAndJoinedAgain(t *testing.T) {
	h := setup(t)
	a, b := token(t, uuid.New()), token(t, uuid.New())
	doJSON(t, h, http.MethodPost, "/v1/injury/groups/jumpers-knee/join", a, joinBody("rehab"))
	doJSON(t, h, http.MethodPost, "/v1/injury/groups/jumpers-knee/join", b, joinBody("jogging"))
	rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/jumpers-knee/posts", a, map[string]any{"kind": "injury", "title": "抜ける前", "body": "抜ける前の投稿", "body_part": "jumpers-knee"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("post %d %s", rec.Code, rec.Body.String())
	}
	var post struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &post); err != nil {
		t.Fatal(err)
	}

	if r := doJSON(t, h, http.MethodDelete, "/v1/injury/groups/jumpers-knee/me", "", nil); r.Code != http.StatusUnauthorized {
		t.Fatalf("leave without login %d", r.Code)
	}
	if r := doJSON(t, h, http.MethodDelete, "/v1/injury/groups/sprain/me", a, nil); r.Code != http.StatusForbidden {
		t.Fatalf("leave a group never joined %d", r.Code)
	}
	if r := doJSON(t, h, http.MethodDelete, "/v1/injury/groups/jumpers-knee/me", a, nil); r.Code != http.StatusNoContent {
		t.Fatalf("leave %d %s", r.Code, r.Body.String())
	}

	// 抜けるとグループの中は読めなくなり、人数も減る。自分の投稿は、自分なら今までどおり読める。
	if r := doJSON(t, h, http.MethodGet, "/v1/injury/groups/jumpers-knee", a, nil); r.Code != http.StatusForbidden {
		t.Fatalf("group still readable after leaving %d", r.Code)
	}
	if g := getGroup(t, h, "jumpers-knee", b); g.Members != 1 {
		t.Fatalf("members after leaving %+v", g)
	}
	if r := doJSON(t, h, http.MethodGet, "/v1/posts/"+post.ID, a, nil); r.Code != http.StatusOK {
		t.Fatalf("own post after leaving %d", r.Code)
	}
	// 書いた投稿は、グループに残る。
	if feed := listInjury(t, h, "/v1/injury/groups/jumpers-knee/posts", b); feed.Meta.Total != 1 {
		t.Fatalf("post should stay in the group %+v", feed)
	}

	// もう一度入れる。
	if r := doJSON(t, h, http.MethodPost, "/v1/injury/groups/jumpers-knee/join", a, joinBody("resting")); r.Code != http.StatusOK {
		t.Fatalf("join again %d", r.Code)
	}
	if g := getGroup(t, h, "jumpers-knee", a); g.Members != 2 || g.Me.Stage != "resting" {
		t.Fatalf("after joining again %+v", g)
	}
}

func TestInjuryGroupStageChangeAnswers(t *testing.T) {
	h := setup(t)
	pool := openPool(t)
	id := uuid.New()
	a := token(t, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/sprain/join", a, joinBody("resting")); rec.Code != http.StatusOK {
		t.Fatalf("join %d %s", rec.Code, rec.Body.String())
	}

	for name, c := range map[string]struct {
		body  map[string]any
		field string
	}{
		"no actions":          {map[string]any{"stage": "rehab"}, "actions"},
		"unknown action":      {map[string]any{"stage": "rehab", "actions": []string{"yoga"}}, "actions"},
		"nothing with others": {map[string]any{"stage": "rehab", "actions": []string{"nothing", "icing"}}, "actions"},
		"public without note": {map[string]any{"stage": "rehab", "actions": []string{"icing"}, "note_public": true}, "note_public"},
		"note too long":       {map[string]any{"stage": "rehab", "actions": []string{"icing"}, "note": strings.Repeat("あ", 301)}, "note"},
		"same stage":          {map[string]any{"stage": "resting", "actions": []string{"icing"}}, "stage"},
	} {
		rec := doJSON(t, h, http.MethodPatch, "/v1/injury/groups/sprain/me", a, c.body)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"`+c.field+`"`) {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}

	rec := doJSON(t, h, http.MethodPatch, "/v1/injury/groups/sprain/me", a, map[string]any{
		"stage": "rehab", "actions": []string{"icing", "rest", "icing"}, "note": "  最初の3日は冷やすだけにした  ", "note_public": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("change %d %s", rec.Code, rec.Body.String())
	}
	var actions []string
	var note *string
	var public bool
	if err := pool.QueryRow(context.Background(), `
		SELECT actions, note, note_public FROM injury_group_stage_log
		WHERE user_id = $1 AND body_part = 'sprain' ORDER BY created_at DESC LIMIT 1`, id).Scan(&actions, &note, &public); err != nil {
		t.Fatal(err)
	}
	if strings.Join(actions, ",") != "icing,rest" || note == nil || *note != "最初の3日は冷やすだけにした" || !public {
		t.Fatalf("log %v %v %v", actions, note, public)
	}
}

// ケガのグループの顔には、投稿への公開をオンにした人のニックネームだけを出す。
func TestInjuryGroupFacesHidePrivateNicknames(t *testing.T) {
	h := setup(t)
	public, private := token(t, uuid.New()), token(t, uuid.New())

	if rec := doJSON(t, h, http.MethodPatch, "/v1/me", public, map[string]any{"nickname": "こうかい", "nickname_public": true, "event": "100m"}); rec.Code != http.StatusOK {
		t.Fatalf("public nickname %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/me", private, map[string]any{"nickname": "ひみつ", "nickname_public": false, "event": "100m"}); rec.Code != http.StatusOK {
		t.Fatalf("private nickname %d %s", rec.Code, rec.Body.String())
	}
	for _, tok := range []string{public, private} {
		if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/sprain/join", tok, joinBody("rehab")); rec.Code != http.StatusOK {
			t.Fatalf("join %d %s", rec.Code, rec.Body.String())
		}
	}

	g := getGroup(t, h, "sprain", public)
	var faces []string
	for _, stage := range g.Stages {
		faces = append(faces, stage.Faces...)
	}
	if len(faces) != 2 {
		t.Fatalf("faces %v", faces)
	}
	joined := strings.Join(faces, ",")
	if strings.Contains(joined, "ひみつ") || !strings.Contains(joined, "こうかい") {
		t.Fatalf("faces leak private nickname: %v", faces)
	}
}

// 保存したケガの投稿は、グループを抜けると保存一覧からも読めなくなり、入り直すとまた出る。
func TestSavedInjuryPostsNeedMembership(t *testing.T) {
	h := setup(t)
	a, b := token(t, uuid.New()), token(t, uuid.New())
	doJSON(t, h, http.MethodPost, "/v1/injury/groups/jumpers-knee/join", a, joinBody("rehab"))
	doJSON(t, h, http.MethodPost, "/v1/injury/groups/jumpers-knee/join", b, joinBody("rehab"))
	rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/jumpers-knee/posts", a, map[string]any{"kind": "injury", "title": "ひざの経過", "body": "ひざの経過です", "body_part": "jumpers-knee"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("post %d %s", rec.Code, rec.Body.String())
	}
	var post struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &post)
	if r := doJSON(t, h, http.MethodPost, "/v1/posts/"+post.ID+"/saves", b, nil); r.Code != http.StatusNoContent {
		t.Fatalf("save %d %s", r.Code, r.Body.String())
	}
	saved := func(tok string) string {
		t.Helper()
		r := doJSON(t, h, http.MethodGet, "/v1/me/saves", tok, nil)
		if r.Code != http.StatusOK {
			t.Fatalf("saves %d %s", r.Code, r.Body.String())
		}
		return r.Body.String()
	}
	if !strings.Contains(saved(b), post.ID) {
		t.Fatal("saved post missing while joined")
	}

	if r := doJSON(t, h, http.MethodDelete, "/v1/injury/groups/jumpers-knee/me", b, nil); r.Code != http.StatusNoContent {
		t.Fatalf("leave %d", r.Code)
	}
	if body := saved(b); strings.Contains(body, post.ID) || !strings.Contains(body, `"total":0`) {
		t.Fatalf("saved injury post readable after leaving: %s", body)
	}

	doJSON(t, h, http.MethodPost, "/v1/injury/groups/jumpers-knee/join", b, joinBody("jogging"))
	if !strings.Contains(saved(b), post.ID) {
		t.Fatal("saved post missing after joining again")
	}
}
