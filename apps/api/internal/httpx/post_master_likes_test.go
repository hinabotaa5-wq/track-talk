package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// 自分でつけたいいねは、投稿マスターのバッジの数に入れない。自分の30件にいいねを押すだけでバッジが付いてしまう。
func TestPostMasterIgnoresSelfLikes(t *testing.T) {
	h := setup(t)
	authorID, fanID := uuid.New(), uuid.New()
	author, fan := token(t, authorID), token(t, fanID)
	if rec := doJSON(t, h, http.MethodPatch, "/v1/me", author, map[string]any{"nickname": "はしる", "nickname_public": true, "event": "100m"}); rec.Code != http.StatusOK {
		t.Fatalf("nickname %d %s", rec.Code, rec.Body.String())
	}
	doJSON(t, h, http.MethodGet, "/v1/me", fan, nil)
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", author, map[string]any{"title": "練習", "body": "きょうの練習"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("post %d %s", rec.Code, rec.Body.String())
	}
	var post struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &post)
	execSQLArgs(t, `INSERT INTO posts (id, author_id, kind, title, body)
		SELECT gen_random_uuid(), author_id, kind, title, body FROM posts, generate_series(1, 29) WHERE id = $1`, post.ID)
	execSQLArgs(t, `INSERT INTO post_likes (post_id, user_id) SELECT id, author_id FROM posts WHERE author_id = $1`, authorID)
	execSQLArgs(t, `UPDATE posts SET like_count = 1 WHERE author_id = $1`, authorID)

	badge := func() bool {
		rec := doJSON(t, h, http.MethodGet, "/v1/posts/"+post.ID, "", nil)
		return strings.Contains(rec.Body.String(), `"author_post_master":true`)
	}
	if badge() {
		t.Fatal("badge from self-likes only")
	}
	execSQLArgs(t, `INSERT INTO post_likes (post_id, user_id) SELECT id, $2::uuid FROM posts WHERE author_id = $1`, authorID, fanID)
	if !badge() {
		t.Fatal("badge missing with likes from others")
	}
}
