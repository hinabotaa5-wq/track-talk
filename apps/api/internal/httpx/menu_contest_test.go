package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

type menuContestView struct {
	Round *struct {
		Number  int    `json:"number"`
		Phase   string `json:"phase"`
		MyEntry *struct {
			Body string `json:"body"`
		} `json:"my_entry"`
		Finalists []menuFinalist `json:"finalists"`
		MyVote    *string        `json:"my_vote"`
	} `json:"round"`
	NextStartsAt *time.Time `json:"next_starts_at"`
	LastResult   *struct {
		Number    int            `json:"number"`
		Finalists []menuFinalist `json:"finalists"`
	} `json:"last_result"`
}

type menuFinalist struct {
	ID         string  `json:"id"`
	Body       string  `json:"body"`
	AuthorName *string `json:"author_name"`
	Mine       bool    `json:"mine"`
	Votes      *int    `json:"votes"`
	Winner     bool    `json:"winner"`
}

// fakeClock はテストの中で進められる時計。
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Set(s string) {
	v, err := time.ParseInLocation(time.DateTime, s, jst.Zone)
	if err != nil {
		panic(err)
	}
	c.mu.Lock()
	c.now = v
	c.mu.Unlock()
}

func menuContest(t *testing.T, h http.Handler, tok string) menuContestView {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/menu-contest", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("menu contest = %d %s", rec.Code, rec.Body.String())
	}
	var v menuContestView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestMenuContestRound(t *testing.T) {
	clock := &fakeClock{}
	h := setupConfig(t, func(cfg *config.Config) { cfg.Clock = clock.Now })
	admin := adminToken(t, h)
	alice := tokenWithProfile(t, uuid.New(), "あり", true)
	bob := tokenWithProfile(t, uuid.New(), "ぼぶ", false)
	carol := token(t, uuid.New())
	voter := token(t, uuid.New())

	// シーズン前は回がなく、次の開始日だけ返す。応募もできない。
	clock.Set("2026-10-20 12:00:00")
	if v := menuContest(t, h, ""); v.Round != nil || v.NextStartsAt == nil {
		t.Fatalf("off season = %+v", v)
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/menu-contest/entry", alice, map[string]any{"body": "300m×10"}); rec.Code != http.StatusConflict {
		t.Fatalf("off season entry = %d", rec.Code)
	}

	// 応募期間。1人1件で、書き直すと上書きになる。
	clock.Set("2026-11-03 07:00:00")
	if rec := doJSON(t, h, http.MethodPut, "/v1/menu-contest/entry", "", map[string]any{"body": "x"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous entry = %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/menu-contest/entry", alice, map[string]any{"body": strings.Repeat("あ", 101)}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("long entry = %d", rec.Code)
	}
	for tok, body := range map[string]string{alice: "最初の案", bob: "坂ダッシュ50本", carol: "1000m×10"} {
		if rec := doJSON(t, h, http.MethodPut, "/v1/menu-contest/entry", tok, map[string]any{"body": body}); rec.Code != http.StatusOK {
			t.Fatalf("entry = %d %s", rec.Code, rec.Body.String())
		}
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/menu-contest/entry", alice, map[string]any{"body": "  300m×15(レスト1分)  "}); rec.Code != http.StatusOK {
		t.Fatalf("rewrite = %d", rec.Code)
	}
	v := menuContest(t, h, alice)
	if v.Round == nil || v.Round.Number != 1 || v.Round.Phase != "entry" || v.Round.MyEntry == nil || v.Round.MyEntry.Body != "300m×15(レスト1分)" {
		t.Fatalf("entry view = %+v", v.Round)
	}
	// 応募中は運営もまだ選べない。
	if rec := doJSON(t, h, http.MethodPut, "/v1/admin/menu-contest/finalists", admin, map[string]any{"entry_ids": []string{uuid.NewString()}}); rec.Code != http.StatusConflict {
		t.Fatalf("pick during entry = %d", rec.Code)
	}

	// 締め切り後(木曜)。応募は閉じ、運営が一覧を読んでトップ3を選ぶ。
	clock.Set("2026-11-12 09:00:00")
	if rec := doJSON(t, h, http.MethodPut, "/v1/menu-contest/entry", alice, map[string]any{"body": "x"}); rec.Code != http.StatusConflict {
		t.Fatalf("late entry = %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/menu-contest", alice, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin list = %d", rec.Code)
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/admin/menu-contest", admin, nil)
	var adminView struct {
		Entries []struct {
			ID         string  `json:"id"`
			Body       string  `json:"body"`
			AuthorName *string `json:"author_name"`
		} `json:"entries"`
		CanPick bool `json:"can_pick"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &adminView)
	if rec.Code != http.StatusOK || len(adminView.Entries) != 3 || !adminView.CanPick {
		t.Fatalf("admin view = %d %s", rec.Code, rec.Body.String())
	}
	ids := map[string]string{}
	for _, e := range adminView.Entries {
		ids[e.Body] = e.ID
	}
	aliceID, bobID := ids["300m×15(レスト1分)"], ids["坂ダッシュ50本"]
	if rec := doJSON(t, h, http.MethodPut, "/v1/admin/menu-contest/finalists", admin, map[string]any{"entry_ids": []string{aliceID, uuid.NewString()}}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown pick = %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/admin/menu-contest/finalists", admin, map[string]any{"entry_ids": []string{aliceID, bobID}}); rec.Code != http.StatusNoContent {
		t.Fatalf("pick = %d %s", rec.Code, rec.Body.String())
	}
	// 選んでも、金曜までは公開しない。
	if v := menuContest(t, h, voter); v.Round.Phase != "picking" || len(v.Round.Finalists) != 0 {
		t.Fatalf("thursday = %+v", v.Round)
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/menu-contest/vote", voter, map[string]any{"entry_id": aliceID}); rec.Code != http.StatusConflict {
		t.Fatalf("vote before friday = %d", rec.Code)
	}

	// 金曜から投票。ニックネームは公開している人だけ出る。自分には投票できない。
	clock.Set("2026-11-13 20:00:00")
	v = menuContest(t, h, alice)
	if v.Round.Phase != "voting" || len(v.Round.Finalists) != 2 {
		t.Fatalf("voting = %+v", v.Round)
	}
	for _, f := range v.Round.Finalists {
		if f.Votes != nil {
			t.Fatal("votes must stay hidden while voting")
		}
		switch f.ID {
		case aliceID:
			if !f.Mine || f.AuthorName == nil || *f.AuthorName != "あり" {
				t.Fatalf("alice finalist = %+v", f)
			}
		case bobID:
			if f.Mine || f.AuthorName != nil {
				t.Fatalf("bob finalist = %+v", f)
			}
		}
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/menu-contest/vote", alice, map[string]any{"entry_id": aliceID}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("self vote = %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/menu-contest/vote", voter, map[string]any{"entry_id": ids["1000m×10"]}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("vote for non-finalist = %d", rec.Code)
	}
	// 入れ直しは上書き。
	for _, vote := range []struct{ tok, id string }{{voter, bobID}, {voter, aliceID}, {bob, aliceID}, {carol, bobID}} {
		if rec := doJSON(t, h, http.MethodPut, "/v1/menu-contest/vote", vote.tok, map[string]any{"entry_id": vote.id}); rec.Code != http.StatusOK {
			t.Fatalf("vote = %d %s", rec.Code, rec.Body.String())
		}
	}
	if v := menuContest(t, h, voter); v.Round.MyVote == nil || *v.Round.MyVote != aliceID {
		t.Fatalf("my vote = %+v", v.Round.MyVote)
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/admin/menu-contest/finalists", admin, map[string]any{"entry_ids": []string{bobID}}); rec.Code != http.StatusConflict {
		t.Fatalf("repick after votes = %d", rec.Code)
	}

	// 翌週の月曜。次の回の応募が始まり、前の回の結果が出る。
	clock.Set("2026-11-16 08:00:00")
	v = menuContest(t, h, "")
	if v.Round == nil || v.Round.Number != 2 || v.Round.Phase != "entry" || v.LastResult == nil || v.LastResult.Number != 1 {
		t.Fatalf("next round = %+v %+v", v.Round, v.LastResult)
	}
	for _, f := range v.LastResult.Finalists {
		want := map[string]int{aliceID: 2, bobID: 1}[f.ID]
		if f.Votes == nil || *f.Votes != want || f.Winner != (f.ID == aliceID) {
			t.Fatalf("result = %+v", f)
		}
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/menu-contest/vote", voter, map[string]any{"entry_id": bobID}); rec.Code != http.StatusConflict {
		t.Fatalf("vote after close = %d", rec.Code)
	}
}
