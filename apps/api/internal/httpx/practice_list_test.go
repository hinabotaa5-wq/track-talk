package httpx_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

type practiceListOut struct {
	Posts []struct {
		ID       string `json:"id"`
		Practice struct {
			Prefecture *string `json:"prefecture"`
			Event      string  `json:"event"`
		} `json:"practice"`
	} `json:"posts"`
	Total      int    `json:"total"`
	NextCursor string `json:"next_cursor"`
}

func createPractice(t *testing.T, h http.Handler, tok, prefecture, event string, starts time.Time) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{
		"kind": "practice", "body": "一緒に走りましょう", "venue": "競技場",
		"starts_at": starts.Format(time.RFC3339), "event": event, "capacity": 2, "prefecture": prefecture,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create practice %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.ID
}

func listPractice(t *testing.T, h http.Handler, query string) practiceListOut {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/practice"+query, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list %s: %d %s", query, rec.Code, rec.Body.String())
	}
	var out practiceListOut
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func practiceIDs(out practiceListOut) []string {
	ids := make([]string, len(out.Posts))
	for i, p := range out.Posts {
		ids[i] = p.ID
	}
	return ids
}

func sameIDs(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %v want %v", label, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s: got %v want %v", label, got, want)
		}
	}
}

// 練習相手の一覧は、これからの解散していない募集を日時の早い順に出し、都道府県と種目で絞れる。
// 指定しなければ、すべての都道府県・種目を出す。
func TestPracticeListFiltersByPrefectureAndEvent(t *testing.T) {
	h := setup(t)
	host := token(t, uuid.New())
	base := time.Now().Add(24 * time.Hour).Truncate(time.Minute)
	osaka := createPractice(t, h, host, "27", "100m", base)
	tokyo100 := createPractice(t, h, host, "13", "100m", base.Add(24*time.Hour))
	tokyo400 := createPractice(t, h, host, "13", "400m", base.Add(48*time.Hour))
	dissolved := createPractice(t, h, host, "13", "100m", base.Add(72*time.Hour))
	if rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+dissolved+"/practice/dissolve", host, nil); rec.Code != http.StatusOK {
		t.Fatalf("dissolve %d %s", rec.Code, rec.Body.String())
	}

	all := listPractice(t, h, "")
	sameIDs(t, "all", practiceIDs(all), []string{osaka, tokyo100, tokyo400})
	if all.Total != 3 || all.NextCursor != "" {
		t.Fatalf("all total=%d next=%q", all.Total, all.NextCursor)
	}
	if p := all.Posts[0].Practice.Prefecture; p == nil || *p != "27" {
		t.Fatalf("prefecture in list %v", p)
	}

	tokyo := listPractice(t, h, "?prefecture=13")
	sameIDs(t, "tokyo", practiceIDs(tokyo), []string{tokyo100, tokyo400})
	sameIDs(t, "tokyo 100m", practiceIDs(listPractice(t, h, "?prefecture=13&event=100m")), []string{tokyo100})
	sameIDs(t, "100m", practiceIDs(listPractice(t, h, "?event=100m")), []string{osaka, tokyo100})
	if got := listPractice(t, h, "?prefecture=01"); got.Total != 0 || len(got.Posts) != 0 {
		t.Fatalf("hokkaido %+v", got)
	}

	for _, query := range []string{"?prefecture=00", "?prefecture=48", "?prefecture=x", "?event=sprint", "?cursor=bad"} {
		if rec := doJSON(t, h, http.MethodGet, "/v1/practice"+query, "", nil); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", query, rec.Code, rec.Body.String())
		}
	}
}

func TestPracticeListPagesByCursor(t *testing.T) {
	h := setup(t)
	base := time.Now().Add(24 * time.Hour).Truncate(time.Minute)
	want := []string{}
	for i := 0; i < 26; i++ {
		// 書き込みの回数制限に当たらないよう、募集する人を分ける。
		want = append(want, createPractice(t, h, token(t, uuid.New()), "13", "100m", base.Add(time.Duration(i)*time.Hour)))
	}
	first := listPractice(t, h, "")
	if first.Total != 26 || len(first.Posts) != 24 || first.NextCursor == "" {
		t.Fatalf("first total=%d len=%d next=%q", first.Total, len(first.Posts), first.NextCursor)
	}
	second := listPractice(t, h, "?cursor="+first.NextCursor)
	if len(second.Posts) != 2 || second.NextCursor != "" {
		t.Fatalf("second len=%d next=%q", len(second.Posts), second.NextCursor)
	}
	sameIDs(t, "pages", append(practiceIDs(first), practiceIDs(second)...), want)
}

// 新しい募集には都道府県が必要。
func TestPracticeRequiresPrefecture(t *testing.T) {
	h := setup(t)
	tok := token(t, uuid.New())
	starts := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
	for _, prefecture := range []any{nil, "", "00", "48"} {
		body := map[string]any{
			"kind": "practice", "body": "一緒に走りましょう", "venue": "競技場",
			"starts_at": starts, "event": "100m", "capacity": 2,
		}
		if prefecture != nil {
			body["prefecture"] = prefecture
		}
		rec := doJSON(t, h, http.MethodPost, "/v1/posts", tok, body)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("prefecture %v: %d %s", prefecture, rec.Code, rec.Body.String())
		}
		var out struct {
			Error struct {
				Fields map[string]string `json:"fields"`
			} `json:"error"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		if out.Error.Fields["prefecture"] == "" {
			t.Fatalf("prefecture %v: no field error %s", prefecture, rec.Body.String())
		}
	}
}
