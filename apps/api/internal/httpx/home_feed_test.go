package httpx_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type homeFeedResponse struct {
	Posts []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	} `json:"posts"`
	Meta struct {
		HasMore bool   `json:"has_more"`
		Next    string `json:"next_cursor"`
		Total   int    `json:"total"`
	} `json:"meta"`
}

func TestHomeFeedCursor(t *testing.T) {
	for _, sort := range []string{"new", "popular"} {
		t.Run(sort, func(t *testing.T) {
			h := setup(t)
			ctx := context.Background()
			pool, err := pgxpool.New(ctx, testDSN(t))
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			author := uuid.New()
			if _, err := pool.Exec(ctx, `INSERT INTO profiles(id,nickname) VALUES($1,'cursor')`, author); err != nil {
				t.Fatal(err)
			}
			stamp := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
			for i := 1; i <= 29; i++ {
				kind := "text"
				if i == 28 {
					kind = "injury"
				}
				if i == 29 {
					kind = "meet"
				}
				id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i)
				if _, err := pool.Exec(ctx, `INSERT INTO posts(id,author_id,title,body,kind,created_at,like_count,hidden_at,deleted_at) VALUES($1,$2,'陸上','本文',$3,$4,5,CASE WHEN $5 THEN now() END,CASE WHEN $6 THEN now() END)`, id, author, kind, stamp, i == 26, i == 27); err != nil {
					t.Fatal(err)
				}
			}
			base := "/v1/posts?pagination=cursor&per_page=12&sort=" + sort + "&q=" + url.QueryEscape("陸上")
			read := func(path string) homeFeedResponse {
				t.Helper()
				rec := doJSON(t, h, http.MethodGet, path, "", nil)
				if rec.Code != 200 {
					t.Fatalf("read %d %s", rec.Code, rec.Body.String())
				}
				var result homeFeedResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				return result
			}
			first := read(base)
			if len(first.Posts) != 12 || !first.Meta.HasMore || first.Meta.Next == "" || first.Meta.Total != 0 {
				t.Fatalf("first %+v", first)
			}
			// 閲覧を始めた後の投稿は、続きを取得するときに混ざらない。
			if _, err := pool.Exec(ctx, `INSERT INTO posts(id,author_id,title,body,kind,created_at,like_count) VALUES($1,$2,'陸上','新規','text',now(),5)`, uuid.New(), author); err != nil {
				t.Fatal(err)
			}
			seen := map[string]bool{}
			result := first
			lastID := "z"
			for page := 1; ; page++ {
				for _, post := range result.Posts {
					if seen[post.ID] || post.ID >= lastID || post.Kind != "text" {
						t.Fatalf("unexpected post %+v", post)
					}
					seen[post.ID] = true
					lastID = post.ID
				}
				if !result.Meta.HasMore {
					if result.Meta.Next != "" {
						t.Fatal("cursor at end")
					}
					break
				}
				if page > 3 {
					t.Fatal("never terminates")
				}
				result = read(base + "&cursor=" + url.QueryEscape(result.Meta.Next))
			}
			if len(seen) != 25 {
				t.Fatalf("got %d posts", len(seen))
			}
			for _, path := range []string{base + "&cursor=invalid", strings.Replace(base, "q="+url.QueryEscape("陸上"), "q=other", 1) + "&cursor=" + url.QueryEscape(first.Meta.Next), base + "&cursor=" + url.QueryEscape(first.Meta.Next) + "&kind=recruitment"} {
				if rec := doJSON(t, h, http.MethodGet, path, "", nil); rec.Code != 400 {
					t.Fatalf("invalid cursor %d %s", rec.Code, rec.Body.String())
				}
			}
			// 従来のページ取得は総件数を返し続ける。
			legacy := read("/v1/posts?sort=" + sort + "&page=2&per_page=12")
			if len(legacy.Posts) != 12 || legacy.Meta.Total != 26 {
				t.Fatalf("legacy %+v", legacy)
			}
		})
	}
}

func TestHomeFeedCursorRecruitmentAndEmpty(t *testing.T) {
	h := setup(t)
	author := token(t, uuid.New())
	// 既存の練習作成ルートを使って、募集タブの付加情報も検証する。
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", author, map[string]any{"kind": "practice", "body": "練習募集", "venue": "国立競技場", "starts_at": time.Now().Add(72 * time.Hour).Format(time.RFC3339), "event": "100m", "capacity": 2})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/posts?pagination=cursor&kind=recruitment&q="+url.QueryEscape("国立"), "", nil)
	if rec.Code != 200 {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}
	var result struct {
		Posts []struct {
			Kind     string          `json:"kind"`
			Practice json.RawMessage `json:"practice"`
		} `json:"posts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Posts) != 1 || result.Posts[0].Kind != "practice" || len(result.Posts[0].Practice) == 0 {
		t.Fatalf("summary %s", rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/posts?pagination=cursor&q=nonexistent", "", nil)
	var empty homeFeedResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &empty)
	if rec.Code != 200 || len(empty.Posts) != 0 || empty.Meta.HasMore || empty.Meta.Next != "" {
		t.Fatalf("empty %s", rec.Body.String())
	}
}

// 自分の投稿もカーソルで続きを読める。同じ時刻やいいね数の投稿も、重複も抜けもなく辿れる。
func TestMyPostsCursor(t *testing.T) {
	for _, sort := range []string{"new", "popular"} {
		t.Run(sort, func(t *testing.T) {
			h := setup(t)
			ctx := context.Background()
			pool, err := pgxpool.New(ctx, testDSN(t))
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			author, other := uuid.New(), uuid.New()
			for _, id := range []uuid.UUID{author, other} {
				if _, err := pool.Exec(ctx, `INSERT INTO profiles(id,nickname) VALUES($1,'mine')`, id); err != nil {
					t.Fatal(err)
				}
			}
			stamp := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
			for i := 1; i <= 23; i++ {
				// 半分は同じ時刻・同じいいね数にして、並びの同点を作る。
				at, likes := stamp, 3
				if i%2 == 0 {
					at, likes = stamp.Add(-time.Duration(i)*time.Minute), i
				}
				if _, err := pool.Exec(ctx, `INSERT INTO posts(id,author_id,title,body,kind,created_at,like_count) VALUES($1,$2,'自分','本文','text',$3,$4)`, uuid.New(), author, at, likes); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := pool.Exec(ctx, `INSERT INTO posts(id,author_id,title,body,kind) VALUES($1,$2,'他人','本文','text')`, uuid.New(), other); err != nil {
				t.Fatal(err)
			}
			tok := token(t, author)
			base := "/v1/me/posts?pagination=cursor&per_page=10&sort=" + sort
			read := func(path string) homeFeedResponse {
				t.Helper()
				rec := doJSON(t, h, http.MethodGet, path, tok, nil)
				if rec.Code != 200 {
					t.Fatalf("read %d %s", rec.Code, rec.Body.String())
				}
				var result homeFeedResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				return result
			}
			seen := map[string]bool{}
			result := read(base)
			first := result.Meta.Next
			for page := 1; ; page++ {
				for _, post := range result.Posts {
					if seen[post.ID] {
						t.Fatalf("duplicate %s", post.ID)
					}
					seen[post.ID] = true
				}
				if !result.Meta.HasMore {
					break
				}
				if page > 3 {
					t.Fatal("never terminates")
				}
				result = read(base + "&cursor=" + url.QueryEscape(result.Meta.Next))
			}
			if len(seen) != 23 {
				t.Fatalf("got %d posts", len(seen))
			}
			// 公開ホームや別の並び順のカーソルは使えない。
			otherSort := map[string]string{"new": "popular", "popular": "new"}[sort]
			for _, path := range []string{
				"/v1/me/posts?pagination=cursor&per_page=10&sort=" + otherSort + "&cursor=" + url.QueryEscape(first),
				"/v1/posts?pagination=cursor&per_page=10&sort=" + sort + "&cursor=" + url.QueryEscape(first),
			} {
				if rec := doJSON(t, h, http.MethodGet, path, tok, nil); rec.Code != 400 {
					t.Fatalf("foreign cursor %s: %d %s", path, rec.Code, rec.Body.String())
				}
			}
			// ページ番号での取得は総件数を返し続ける。
			legacy := read("/v1/me/posts?sort=" + sort + "&page=1&per_page=10")
			if legacy.Meta.Total != 23 {
				t.Fatalf("legacy %+v", legacy.Meta)
			}
		})
	}
}
