package httpx_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPersonalPagesAreBoundedAndPrivate(t *testing.T) {
	for _, collection := range []string{"saves", "injury-history"} {
		t.Run(collection, func(t *testing.T) {
			h := setup(t)
			ctx := context.Background()
			pool, err := pgxpool.New(ctx, testDSN(t))
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			mine, other := uuid.New(), uuid.New()
			for _, user := range []uuid.UUID{mine, other} {
				if _, err := pool.Exec(ctx, `INSERT INTO profiles(id,nickname) VALUES($1,'runner')`, user); err != nil {
					t.Fatal(err)
				}
			}
			stamp := time.Now().Add(-time.Hour)
			for i := 1; i <= 29; i++ {
				id := uuid.MustParse(fmt.Sprintf("00000000-0000-4000-8000-%012d", i))
				author := mine
				if i == 29 {
					author = other
				}
				kind := "text"
				if collection == "injury-history" {
					kind = "injury"
				}
				if _, err := pool.Exec(ctx, `INSERT INTO posts(id,author_id,title,body,kind,created_at,hidden_at,deleted_at) VALUES($1,$2,'記録','本文',$3,$4,CASE WHEN $5 THEN now() END,CASE WHEN $6 THEN now() END)`, id, author, kind, stamp, i == 27, i == 28); err != nil {
					t.Fatal(err)
				}
				if collection == "saves" {
					if _, err := pool.Exec(ctx, `INSERT INTO post_saves(post_id,user_id,created_at) VALUES($1,$2,$3)`, id, author, stamp); err != nil {
						t.Fatal(err)
					}
				} else {
					if _, err := pool.Exec(ctx, `INSERT INTO injury_details(post_id,body_part) VALUES($1,'quad-strain')`, id); err != nil {
						t.Fatal(err)
					}
					if _, err := pool.Exec(ctx, `INSERT INTO injury_updates(id,post_id,stage,body,created_at) VALUES($1,$2,'rehab','歩ける',$3)`, uuid.New(), id, stamp.Add(24*time.Hour)); err != nil {
						t.Fatal(err)
					}
				}
			}
			base := "/v1/me/" + collection + "?pagination=cursor&per_page=12"
			if rec := doJSON(t, h, http.MethodGet, base, "", nil); rec.Code != 401 {
				t.Fatalf("anonymous: %d", rec.Code)
			}
			userToken := token(t, mine)
			seen := map[string]bool{}
			next := ""
			firstCursor := ""
			for page := 0; page < 4; page++ {
				path := base
				if next != "" {
					path += "&cursor=" + url.QueryEscape(next)
				}
				rec := doJSON(t, h, http.MethodGet, path, userToken, nil)
				if rec.Code != 200 {
					t.Fatalf("page %d: %d %s", page, rec.Code, rec.Body.String())
				}
				var got struct {
					Posts []struct {
						ID string `json:"id"`
					} `json:"posts"`
					Items []struct {
						ID     string `json:"id"`
						Stages []any  `json:"stages"`
					} `json:"items"`
					Repeated []struct {
						Count int `json:"count"`
					} `json:"repeated"`
					Meta struct {
						More  bool   `json:"has_more"`
						Next  string `json:"next_cursor"`
						Total int    `json:"total"`
					} `json:"meta"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				ids := []string{}
				for _, post := range got.Posts {
					ids = append(ids, post.ID)
				}
				for _, post := range got.Items {
					ids = append(ids, post.ID)
					if len(post.Stages) != 1 {
						t.Fatal("lost stage history")
					}
				}
				if len(ids) > 12 || got.Meta.Total != 0 {
					t.Fatalf("unbounded response %s", rec.Body.String())
				}
				if collection == "injury-history" && (len(got.Repeated) != 1 || got.Repeated[0].Count != 26) {
					t.Fatalf("repeated count should include all pages: %s", rec.Body.String())
				}
				for _, id := range ids {
					if seen[id] {
						t.Fatal("duplicate across pages")
					}
					seen[id] = true
				}
				if page == 0 {
					firstCursor = got.Meta.Next
				}
				next = got.Meta.Next
				if !got.Meta.More {
					if next != "" {
						t.Fatal("cursor past end")
					}
					break
				}
			}
			if len(seen) != 26 {
				t.Fatalf("got %d records", len(seen))
			}
			for _, check := range []struct{ path, tok string }{
				{base + "&cursor=bad", userToken},
				{base + "&cursor=" + url.QueryEscape(firstCursor), token(t, other)},
			} {
				if rec := doJSON(t, h, http.MethodGet, check.path, check.tok, nil); rec.Code != 400 {
					t.Fatalf("invalid cursor accepted: %d", rec.Code)
				}
			}
		})
	}
}
