package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
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

func TestInjuryFeedFilterByLatestStage(t *testing.T) {
	h := setup(t)
	tok := token(t, uuid.New())

	create := func(title string) string {
		t.Helper()
		rec := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{
			"kind": "injury", "title": title, "body": "本文", "mood": "vent", "body_part": "knee",
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
		return post.ID
	}
	report := func(id, stage string) {
		t.Helper()
		r := doJSON(t, h, http.MethodPost, "/v1/posts/"+id+"/injury/updates", tok, map[string]any{"stage": stage, "body": "経過 " + stage})
		if r.Code != http.StatusCreated {
			t.Fatalf("update %d %s", r.Code, r.Body.String())
		}
	}

	back := create("復帰した")
	report(back, "rehab")
	report(back, "race")
	resting := create("まだ安静")
	report(resting, "resting")
	create("経過なし")

	base := "/v1/posts?kind=injury&sort=new&per_page=20"
	if got := listInjury(t, h, base, "").Meta.Total; got != 3 {
		t.Fatalf("no filter total %d", got)
	}
	race := listInjury(t, h, base+"&stage=race", "")
	if race.Meta.Total != 1 || len(race.Posts) != 1 || race.Posts[0].ID != back {
		t.Fatalf("stage=race %+v", race)
	}
	// 途中の段階(rehab)は最新ではないので一致しない。
	if got := listInjury(t, h, base+"&stage=rehab", "").Meta.Total; got != 0 {
		t.Fatalf("stage=rehab total %d", got)
	}
	// 不正な値は無視されて絞り込まれない。
	if got := listInjury(t, h, base+"&stage=flying", "").Meta.Total; got != 3 {
		t.Fatalf("invalid stage total %d", got)
	}
}

type injuryStatsResp struct {
	MinSample int `json:"min_sample"`
	Parts     []struct {
		BodyPart string `json:"body_part"`
		Posts    int64  `json:"posts"`
		Stages   []struct {
			Stage      string `json:"stage"`
			Reached    int64  `json:"reached"`
			MedianDays *int   `json:"median_days"`
		} `json:"stages"`
	} `json:"parts"`
}

func getInjuryStats(t *testing.T, h http.Handler) injuryStatsResp {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/injury/stats", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats %d %s", rec.Code, rec.Body.String())
	}
	var out injuryStatsResp
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// reached returns the post count and the reached count for a part and stage (0 when absent).
func (r injuryStatsResp) reached(part, stage string) (posts, reached int64, median *int) {
	for _, p := range r.Parts {
		if p.BodyPart != part {
			continue
		}
		posts = p.Posts
		for _, s := range p.Stages {
			if s.Stage == stage {
				return posts, s.Reached, s.MedianDays
			}
		}
	}
	return posts, 0, nil
}

func TestInjuryStatsAggregatesAnonymously(t *testing.T) {
	h := setup(t)
	// DB はテスト間で共有されるので、増えた分で確かめる。
	beforePosts, beforeReached, _ := getInjuryStats(t, h).reached("groin", "jogging")

	for i := 0; i < 5; i++ {
		tok := token(t, uuid.New())
		rec := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{
			"kind": "injury", "title": "鼠径部", "body": "本文", "mood": "rehab", "body_part": "groin",
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
		up := doJSON(t, h, http.MethodPost, "/v1/posts/"+post.ID+"/injury/updates", tok, map[string]any{"stage": "jogging", "body": "走れた"})
		if up.Code != http.StatusCreated {
			t.Fatalf("update %d %s", up.Code, up.Body.String())
		}
	}

	stats := getInjuryStats(t, h)
	if stats.MinSample != 5 {
		t.Fatalf("min sample %d", stats.MinSample)
	}
	posts, reached, median := stats.reached("groin", "jogging")
	if posts-beforePosts != 5 || reached-beforeReached != 5 {
		t.Fatalf("delta posts %d reached %d", posts-beforePosts, reached-beforeReached)
	}
	if median == nil || *median < 0 {
		t.Fatalf("median should be shown once the sample is large enough: %v", median)
	}
}

func TestMyInjuryHistoryIsPrivateToTheAuthor(t *testing.T) {
	h := setup(t)
	mine := token(t, uuid.New())
	other := token(t, uuid.New())

	create := func(tok, title string) string {
		t.Helper()
		rec := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{
			"kind": "injury", "title": title, "body": "本文", "mood": "vent", "body_part": "calf",
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
		return post.ID
	}
	id := create(mine, "ふくらはぎの肉離れ")
	create(other, "他人のケガ")
	if r := doJSON(t, h, http.MethodPost, "/v1/posts/"+id+"/injury/updates", mine, map[string]any{"stage": "rehab", "body": "歩ける"}); r.Code != http.StatusCreated {
		t.Fatalf("update %d %s", r.Code, r.Body.String())
	}
	// ケガ以外の投稿は履歴に出ない。
	if r := doJSON(t, h, http.MethodPost, "/v1/posts", mine, map[string]any{"kind": "text", "title": "ふつう", "body": "本文"}); r.Code != http.StatusCreated {
		t.Fatalf("create text %d", r.Code)
	}

	if r := doJSON(t, h, http.MethodGet, "/v1/me/injury-history", "", nil); r.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status %d", r.Code)
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/me/injury-history", mine, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("history %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Items []struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Stages []struct {
				Stage string `json:"stage"`
				Days  int    `json:"days"`
			} `json:"stages"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.Items[0].ID != id || got.Items[0].Title != "ふくらはぎの肉離れ" {
		t.Fatalf("history %s", rec.Body.String())
	}
	if len(got.Items[0].Stages) != 1 || got.Items[0].Stages[0].Stage != "rehab" || got.Items[0].Stages[0].Days != 0 {
		t.Fatalf("stages %s", rec.Body.String())
	}
}

func TestInjuryUpdateNotifiesLikersAndSavers(t *testing.T) {
	h := setup(t)
	author := token(t, uuid.New())
	liker := token(t, uuid.New())
	saver := token(t, uuid.New())
	bystander := token(t, uuid.New())

	rec := doJSON(t, h, http.MethodPost, "/v1/posts", author, map[string]any{
		"kind": "injury", "title": "通知の確認", "body": "本文", "mood": "vent", "body_part": "hip",
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
	// 投稿者本人の「わかる」は対象外。
	for _, tok := range []string{liker, author} {
		if r := doJSON(t, h, http.MethodPost, "/v1/posts/"+post.ID+"/likes", tok, nil); r.Code >= 300 {
			t.Fatalf("like %d %s", r.Code, r.Body.String())
		}
	}
	if r := doJSON(t, h, http.MethodPost, "/v1/posts/"+post.ID+"/saves", saver, nil); r.Code >= 300 {
		t.Fatalf("save %d %s", r.Code, r.Body.String())
	}

	notes := func(tok string) []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
		Read bool   `json:"read"`
	} {
		t.Helper()
		r := doJSON(t, h, http.MethodGet, "/v1/me/notifications", tok, nil)
		if r.Code != http.StatusOK {
			t.Fatalf("notifications %d %s", r.Code, r.Body.String())
		}
		var out struct {
			Notifications []struct {
				ID   string `json:"id"`
				Kind string `json:"kind"`
				Read bool   `json:"read"`
			} `json:"notifications"`
		}
		if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out.Notifications
	}
	report := func(stage string) {
		t.Helper()
		r := doJSON(t, h, http.MethodPost, "/v1/posts/"+post.ID+"/injury/updates", author, map[string]any{"stage": stage, "body": "経過 " + stage})
		if r.Code != http.StatusCreated {
			t.Fatalf("update %d %s", r.Code, r.Body.String())
		}
	}

	report("rehab")
	for name, tok := range map[string]string{"liker": liker, "saver": saver} {
		got := notes(tok)
		if len(got) != 1 || got[0].Kind != "injury_update" {
			t.Fatalf("%s notifications %+v", name, got)
		}
	}
	if got := notes(author); len(got) != 0 {
		t.Fatalf("author must not be notified of their own update: %+v", got)
	}
	if got := notes(bystander); len(got) != 0 {
		t.Fatalf("bystander notified: %+v", got)
	}

	// 未読のあいだは重ねて通知しない。
	report("jogging")
	if got := notes(liker); len(got) != 1 {
		t.Fatalf("unread notification must not be duplicated: %+v", got)
	}
	// 読んだあとの次の報告は、あらためて通知する。
	first := notes(liker)[0]
	if r := doJSON(t, h, http.MethodPost, "/v1/me/notifications/"+first.ID+"/read", liker, nil); r.Code >= 300 {
		t.Fatalf("read %d %s", r.Code, r.Body.String())
	}
	report("practice")
	if got := notes(liker); len(got) != 2 {
		t.Fatalf("a new update after reading should notify again: %+v", got)
	}
}

func TestInjuryUpdateCanBeEditedByTheAuthorOnly(t *testing.T) {
	h := setup(t)
	author := token(t, uuid.New())
	other := token(t, uuid.New())
	follower := token(t, uuid.New())

	rec := doJSON(t, h, http.MethodPost, "/v1/posts", author, map[string]any{
		"kind": "injury", "title": "編集の確認", "body": "本文", "mood": "vent", "body_part": "upper-body",
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
	if r := doJSON(t, h, http.MethodPost, "/v1/posts/"+post.ID+"/saves", follower, nil); r.Code >= 300 {
		t.Fatalf("save %d", r.Code)
	}
	created := doJSON(t, h, http.MethodPost, "/v1/posts/"+post.ID+"/injury/updates", author, map[string]any{"stage": "rehab", "body": "最初の報告"})
	if created.Code != http.StatusCreated {
		t.Fatalf("update %d %s", created.Code, created.Body.String())
	}
	var up struct {
		ID        string `json:"id"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &up); err != nil {
		t.Fatal(err)
	}
	path := "/v1/posts/" + post.ID + "/injury/updates/" + up.ID
	fix := map[string]any{"stage": "jogging", "body": "書き直した報告"}

	if r := doJSON(t, h, http.MethodPatch, path, "", fix); r.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status %d", r.Code)
	}
	if r := doJSON(t, h, http.MethodPatch, path, other, fix); r.Code != http.StatusNotFound {
		t.Fatalf("other user status %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, http.MethodPatch, path, author, map[string]any{"stage": "flying", "body": "x"}); r.Code == http.StatusOK {
		t.Fatalf("invalid stage accepted")
	}
	if r := doJSON(t, h, http.MethodPatch, path, author, map[string]any{"stage": "rehab", "body": "  "}); r.Code == http.StatusOK {
		t.Fatalf("empty body accepted")
	}
	missing := "/v1/posts/" + post.ID + "/injury/updates/" + uuid.NewString()
	if r := doJSON(t, h, http.MethodPatch, missing, author, fix); r.Code != http.StatusNotFound {
		t.Fatalf("unknown update status %d", r.Code)
	}

	r := doJSON(t, h, http.MethodPatch, path, author, fix)
	if r.Code != http.StatusOK {
		t.Fatalf("edit %d %s", r.Code, r.Body.String())
	}
	var edited struct {
		ID         string `json:"id"`
		Stage      string `json:"stage"`
		StageLabel string `json:"stage_label"`
		Body       string `json:"body"`
		CreatedAt  string `json:"created_at"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &edited); err != nil {
		t.Fatal(err)
	}
	if edited.ID != up.ID || edited.Stage != "jogging" || edited.StageLabel != "ジョグ再開" || edited.Body != "書き直した報告" {
		t.Fatalf("edited %+v", edited)
	}
	if edited.CreatedAt != up.CreatedAt {
		t.Fatalf("created_at must not change: %s -> %s", up.CreatedAt, edited.CreatedAt)
	}

	// フィードの最新の段階にも反映される。
	feed := doJSON(t, h, http.MethodGet, "/v1/posts?kind=injury&sort=new&stage=jogging", "", nil)
	if feed.Code != http.StatusOK || !strings.Contains(feed.Body.String(), post.ID) {
		t.Fatalf("edited stage should appear in the stage filter: %d %s", feed.Code, feed.Body.String())
	}
	// 編集では通知を送らない(保存した人への通知は最初の報告の1件のまま)。
	nr := doJSON(t, h, http.MethodGet, "/v1/me/notifications", follower, nil)
	var notes struct {
		Notifications []struct {
			Kind string `json:"kind"`
		} `json:"notifications"`
	}
	if err := json.Unmarshal(nr.Body.Bytes(), &notes); err != nil {
		t.Fatal(err)
	}
	if len(notes.Notifications) != 1 {
		t.Fatalf("edit must not notify again: %+v", notes.Notifications)
	}
}
