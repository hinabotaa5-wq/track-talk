package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCommentCountsFollowHideAndDelete(t *testing.T) {
	h := setup(t)
	author := token(t, uuid.New())
	commenter := token(t, uuid.New())
	adminID := uuid.New()
	admin := token(t, adminID)
	postID := createTextPost(t, h, author, "件数")

	if rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+postID+"/comments", commenter, map[string]string{"body": "最初"}); rec.Code != http.StatusCreated {
		t.Fatalf("comment %d %s", rec.Code, rec.Body.String())
	}
	if likes, comments := postCounts(t, h, postID); likes != 0 || comments != 1 {
		t.Fatalf("after comment likes=%d comments=%d", likes, comments)
	}

	var created struct {
		ID string `json:"id"`
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+postID+"/comments", commenter, map[string]string{"body": "消す"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("second %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if rec := doJSON(t, h, http.MethodDelete, "/v1/comments/"+created.ID, commenter, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}
	if _, comments := postCounts(t, h, postID); comments != 1 {
		t.Fatalf("after delete comments=%d", comments)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+postID+"/comments", commenter, map[string]string{"body": "隠す"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("third %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("admin profile %d %s", rec.Code, rec.Body.String())
	}
	setAdmin(t, adminID)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/comments/"+created.ID+"/hide", admin, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("hide %d %s", rec.Code, rec.Body.String())
	}
	if _, comments := postCounts(t, h, postID); comments != 1 {
		t.Fatalf("after hide comments=%d", comments)
	}
	if total := commentTotal(t, h, postID); total != 1 {
		t.Fatalf("visible comments %d", total)
	}
}

func TestLikeCountRollsBackWithTheLike(t *testing.T) {
	h := setup(t)
	author := token(t, uuid.New())
	liker := token(t, uuid.New())
	postID := createTextPost(t, h, author, "いいね")
	installTrigger(t, `
CREATE OR REPLACE FUNCTION fail_like_count() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.like_count IS DISTINCT FROM OLD.like_count THEN
    RAISE EXCEPTION 'like count failed';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER fail_like_count
BEFORE UPDATE OF like_count ON posts
FOR EACH ROW EXECUTE FUNCTION fail_like_count();
`, `DROP TRIGGER IF EXISTS fail_like_count ON posts; DROP FUNCTION IF EXISTS fail_like_count();`)

	if rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+postID+"/likes", liker, nil); rec.Code != http.StatusInternalServerError {
		t.Fatalf("like %d %s", rec.Code, rec.Body.String())
	}
	likes, liked := likeState(t, h, postID, liker)
	if likes != 0 || liked {
		t.Fatalf("rolled back likes=%d liked=%v", likes, liked)
	}

	execSQL(t, `DROP TRIGGER IF EXISTS fail_like_count ON posts; DROP FUNCTION IF EXISTS fail_like_count();`)
	if rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+postID+"/likes", liker, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("like again %d %s", rec.Code, rec.Body.String())
	}
	likes, liked = likeState(t, h, postID, liker)
	if likes != 1 || !liked {
		t.Fatalf("after like likes=%d liked=%v", likes, liked)
	}
	if rec := doJSON(t, h, http.MethodDelete, "/v1/posts/"+postID+"/likes", liker, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("unlike %d %s", rec.Code, rec.Body.String())
	}
	likes, liked = likeState(t, h, postID, liker)
	if likes != 0 || liked {
		t.Fatalf("after unlike likes=%d liked=%v", likes, liked)
	}
}

func TestCommentRollsBackWhenNotifyFails(t *testing.T) {
	h := setup(t)
	authorID := uuid.New()
	author := token(t, authorID)
	commenter := token(t, uuid.New())
	postID := createTextPost(t, h, author, "通知")
	installTrigger(t, `
CREATE OR REPLACE FUNCTION fail_comment_notify() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.kind = 'comment' THEN
    RAISE EXCEPTION 'notify failed';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER fail_comment_notify
BEFORE INSERT ON notifications
FOR EACH ROW EXECUTE FUNCTION fail_comment_notify();
`, `DROP TRIGGER IF EXISTS fail_comment_notify ON notifications; DROP FUNCTION IF EXISTS fail_comment_notify();`)

	if rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+postID+"/comments", commenter, map[string]string{"body": "残らない"}); rec.Code != http.StatusInternalServerError {
		t.Fatalf("comment %d %s", rec.Code, rec.Body.String())
	}
	if _, comments := postCounts(t, h, postID); comments != 0 {
		t.Fatalf("count after failed notify %d", comments)
	}
	if total := commentTotal(t, h, postID); total != 0 {
		t.Fatalf("comments after failed notify %d", total)
	}
	notes, _ := listNotifications(t, h, author)
	if len(notesFor(notes, postID, "comment")) != 0 {
		t.Fatalf("notification remained %+v", notes)
	}
}

func postCounts(t *testing.T, h http.Handler, postID string) (likes, comments int) {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/posts/"+postID, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		LikeCount    int `json:"like_count"`
		CommentCount int `json:"comment_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.LikeCount, body.CommentCount
}

func likeState(t *testing.T, h http.Handler, postID, tok string) (likes int, liked bool) {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/posts/"+postID, tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		LikeCount int  `json:"like_count"`
		LikedByMe bool `json:"liked_by_me"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.LikeCount, body.LikedByMe
}

func installTrigger(t *testing.T, create, drop string) {
	t.Helper()
	execSQL(t, create)
	t.Cleanup(func() { execSQL(t, drop) })
}

func execSQL(t *testing.T, query string) {
	t.Helper()
	dsn := testDSN(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, query); err != nil {
		t.Fatal(err)
	}
}
