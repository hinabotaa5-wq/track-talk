package httpx_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type injuryListResp struct {
	Posts []struct {
		ID     string `json:"id"`
		Kind   string `json:"kind"`
		Injury *struct {
			Mood          string `json:"mood"`
			BodyPartLabel string `json:"body_part_label"`
		} `json:"injury"`
	} `json:"posts"`
	Meta struct {
		Total int64 `json:"total"`
	} `json:"meta"`
}

func listInjury(t *testing.T, h http.Handler, path, tok string) injuryListResp {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, path, tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list %s %d %s", path, rec.Code, rec.Body.String())
	}
	var out injuryListResp
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestInjuryPostLifecycleAndFeedSeparation(t *testing.T) {
	h := setup(t)
	author := uuid.New()
	tok := token(t, author)

	bad := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{
		"kind": "injury", "body": "走れない", "mood": "vent", "body_part": "head",
	})
	if bad.Code != http.StatusUnprocessableEntity && bad.Code != http.StatusBadRequest {
		t.Fatalf("invalid part status %d %s", bad.Code, bad.Body.String())
	}
	empty := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{
		"kind": "injury", "body": "  ", "mood": "vent", "body_part": "knee",
	})
	if empty.Code == http.StatusCreated {
		t.Fatalf("empty body accepted")
	}

	rec := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{
		"kind": "injury", "title": "肉離れ", "body": "もう三週間走れていない", "mood": "vent", "body_part": "hamstring",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID     string `json:"id"`
		Kind   string `json:"kind"`
		Injury struct {
			Mood          string `json:"mood"`
			BodyPartLabel string `json:"body_part_label"`
		} `json:"injury"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Kind != "injury" || created.Injury.Mood != "vent" || created.Injury.BodyPartLabel != "ハムストリング" {
		t.Fatalf("unexpected create response %+v", created)
	}
	rehab := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{
		"kind": "injury", "body": "バイクとコアを続ける", "mood": "rehab", "body_part": "knee",
	})
	if rehab.Code != http.StatusCreated {
		t.Fatalf("create rehab %d %s", rehab.Code, rehab.Body.String())
	}

	home := listInjury(t, h, "/v1/posts?sort=new", "")
	for _, p := range home.Posts {
		if p.Kind == "injury" {
			t.Fatalf("injury post leaked into the home feed")
		}
	}
	all := listInjury(t, h, "/v1/posts?kind=injury&sort=new", "")
	if all.Meta.Total != 2 || len(all.Posts) != 2 || all.Posts[0].Injury == nil {
		t.Fatalf("injury feed %+v", all)
	}
	vent := listInjury(t, h, "/v1/posts?kind=injury&mood=vent", "")
	if vent.Meta.Total != 1 || vent.Posts[0].ID != created.ID {
		t.Fatalf("mood filter %+v", vent)
	}
	knee := listInjury(t, h, "/v1/posts?kind=injury&part=knee", "")
	if knee.Meta.Total != 1 || knee.Posts[0].Injury.Mood != "rehab" {
		t.Fatalf("part filter %+v", knee)
	}
	mine := listInjury(t, h, "/v1/me/posts", tok)
	if mine.Meta.Total != 2 {
		t.Fatalf("my posts should include injury posts: %+v", mine)
	}

	patch := doJSON(t, h, http.MethodPatch, "/v1/posts/"+created.ID, tok, map[string]any{
		"title": "肉離れ", "body": "少し歩けた", "mood": "rehab", "body_part": "calf",
	})
	if patch.Code != http.StatusOK {
		t.Fatalf("patch %d %s", patch.Code, patch.Body.String())
	}
	got := doJSON(t, h, http.MethodGet, "/v1/posts/"+created.ID, "", nil)
	var detail struct {
		Body   string `json:"body"`
		Injury struct {
			Mood     string `json:"mood"`
			BodyPart string `json:"body_part"`
		} `json:"injury"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Body != "少し歩けた" || detail.Injury.Mood != "rehab" || detail.Injury.BodyPart != "calf" {
		t.Fatalf("detail after patch %+v", detail)
	}
}

func TestInjuryRecoveryUpdates(t *testing.T) {
	h := setup(t)
	author := uuid.New()
	tok := token(t, author)
	other := token(t, uuid.New())

	rec := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{
		"kind": "injury", "title": "疲労骨折", "body": "診断が出た", "mood": "vent", "body_part": "shin",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var post struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &post); err != nil {
		t.Fatal(err)
	}
	text := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{"kind": "text", "title": "ふつう", "body": "本文"})
	if text.Code != http.StatusCreated {
		t.Fatalf("create text %d %s", text.Code, text.Body.String())
	}
	var textPost struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(text.Body.Bytes(), &textPost); err != nil {
		t.Fatal(err)
	}
	path := "/v1/posts/" + post.ID + "/injury/updates"

	if r := doJSON(t, h, http.MethodPost, path, "", map[string]any{"stage": "rehab", "body": "x"}); r.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status %d", r.Code)
	}
	if r := doJSON(t, h, http.MethodPost, path, other, map[string]any{"stage": "rehab", "body": "x"}); r.Code != http.StatusForbidden {
		t.Fatalf("non-author status %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, http.MethodPost, "/v1/posts/"+textPost.ID+"/injury/updates", tok, map[string]any{"stage": "rehab", "body": "x"}); r.Code != http.StatusNotFound {
		t.Fatalf("non-injury post status %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, http.MethodPost, path, tok, map[string]any{"stage": "flying", "body": "x"}); r.Code == http.StatusCreated {
		t.Fatalf("invalid stage accepted")
	}
	if r := doJSON(t, h, http.MethodPost, path, tok, map[string]any{"stage": "rehab", "body": "  "}); r.Code == http.StatusCreated {
		t.Fatalf("empty body accepted")
	}

	var firstID string
	for i, stage := range []string{"rehab", "jogging"} {
		r := doJSON(t, h, http.MethodPost, path, tok, map[string]any{"stage": stage, "body": "経過 " + stage})
		if r.Code != http.StatusCreated {
			t.Fatalf("update %d status %d %s", i, r.Code, r.Body.String())
		}
		if i == 0 {
			var u struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(r.Body.Bytes(), &u); err != nil {
				t.Fatal(err)
			}
			firstID = u.ID
		}
	}

	got := doJSON(t, h, http.MethodGet, "/v1/posts/"+post.ID, "", nil)
	var detail struct {
		Injury struct {
			Stage      string `json:"stage"`
			StageLabel string `json:"stage_label"`
			Updates    []struct {
				Stage string `json:"stage"`
				Body  string `json:"body"`
			} `json:"updates"`
		} `json:"injury"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Injury.Updates) != 2 || detail.Injury.Updates[0].Stage != "rehab" || detail.Injury.Stage != "jogging" || detail.Injury.StageLabel != "ジョグ再開" {
		t.Fatalf("detail %+v", detail)
	}

	feed := doJSON(t, h, http.MethodGet, "/v1/posts?kind=injury", "", nil)
	var list struct {
		Posts []struct {
			Injury struct {
				Stage   string `json:"stage"`
				Updates []any  `json:"updates"`
			} `json:"injury"`
		} `json:"posts"`
	}
	if err := json.Unmarshal(feed.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Posts) != 1 || list.Posts[0].Injury.Stage != "jogging" || len(list.Posts[0].Injury.Updates) != 0 {
		t.Fatalf("feed should show the latest stage without the update list: %s", feed.Body.String())
	}

	if r := doJSON(t, h, http.MethodDelete, path+"/"+firstID, other, nil); r.Code != http.StatusNotFound {
		t.Fatalf("non-author delete status %d", r.Code)
	}
	if r := doJSON(t, h, http.MethodDelete, path+"/"+firstID, tok, nil); r.Code != http.StatusNoContent {
		t.Fatalf("delete status %d %s", r.Code, r.Body.String())
	}
	got = doJSON(t, h, http.MethodGet, "/v1/posts/"+post.ID, "", nil)
	detail.Injury.Updates = nil
	if err := json.Unmarshal(got.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Injury.Updates) != 1 {
		t.Fatalf("after delete %+v", detail)
	}
}
