package httpx_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type replyComment struct {
	ID             string  `json:"id"`
	Body           string  `json:"body"`
	AuthorName     string  `json:"author_name"`
	ParentID       *string `json:"parent_id"`
	ReplyToID      *string `json:"reply_to_id"`
	ReplyToName    *string `json:"reply_to_name"`
	ReplyToRemoved bool    `json:"reply_to_removed"`
}

func postComment(t *testing.T, h http.Handler, tok, postID, body string, replyTo *string) replyComment {
	t.Helper()
	payload := map[string]any{"body": body}
	if replyTo != nil {
		payload["reply_to_id"] = *replyTo
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+postID+"/comments", tok, payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("comment %d %s", rec.Code, rec.Body.String())
	}
	var c replyComment
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func listReplyComments(t *testing.T, h http.Handler, tok, postID string) map[string]replyComment {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/posts/"+postID+"/comments", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Comments []replyComment `json:"comments"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	out := map[string]replyComment{}
	for _, c := range body.Comments {
		out[c.Body] = c
	}
	return out
}

func TestCommentRepliesAreFlattenedAndLabeled(t *testing.T) {
	h := setup(t)
	authorID, aID, bID := uuid.New(), uuid.New(), uuid.New()
	author := tokenWithProfile(t, authorID, "投稿主", true)
	a := tokenWithProfile(t, aID, "あさひ", true)
	b := tokenWithProfile(t, bID, "ひみつ", false)
	postID := createTextPost(t, h, author, "返信")

	top := postComment(t, h, a, postID, "最上位", nil)
	r1 := postComment(t, h, b, postID, "返信1", &top.ID)
	r2 := postComment(t, h, author, postID, "返信への返信", &r1.ID)

	if r1.ParentID == nil || *r1.ParentID != top.ID || r1.ReplyToID == nil || *r1.ReplyToID != top.ID {
		t.Fatalf("r1 %+v", r1)
	}
	if r2.ParentID == nil || *r2.ParentID != top.ID || r2.ReplyToID == nil || *r2.ReplyToID != r1.ID {
		t.Fatalf("r2 should hang under the top comment: %+v", r2)
	}

	got := listReplyComments(t, h, a, postID)
	if got["最上位"].AuthorName != "あなた" || got["返信1"].AuthorName != "匿名" || got["返信への返信"].AuthorName != "投稿者" {
		t.Fatalf("author labels %+v", got)
	}
	if got["最上位"].ReplyToID != nil {
		t.Fatalf("top comment has reply target %+v", got["最上位"])
	}
	if n := got["返信1"].ReplyToName; n == nil || *n != "あなた" {
		t.Fatalf("viewer's own comment should be あなた: %+v", got["返信1"])
	}
	if n := got["返信への返信"].ReplyToName; n == nil || *n != "匿名" {
		t.Fatalf("private nickname should be 匿名: %+v", got["返信への返信"])
	}
	got = listReplyComments(t, h, b, postID)
	if n := got["返信1"].ReplyToName; n == nil || *n != "匿名" {
		t.Fatalf("public nickname must not be shown on comments: %+v", got["返信1"])
	}
	if n := got["返信への返信"].ReplyToName; n == nil || *n != "あなた" {
		t.Fatalf("viewer should see あなた: %+v", got["返信への返信"])
	}
	// 投稿者宛て
	postComment(t, h, a, postID, "投稿者宛て", &r2.ID)
	got = listReplyComments(t, h, b, postID)
	if n := got["投稿者宛て"].ReplyToName; n == nil || *n != "投稿者" {
		t.Fatalf("post author label expected: %+v", got["投稿者宛て"])
	}
}

func TestCommentReplyValidation(t *testing.T) {
	h := setup(t)
	author := token(t, uuid.New())
	other := token(t, uuid.New())
	postA := createTextPost(t, h, author, "A")
	postB := createTextPost(t, h, author, "B")
	onA := postComment(t, h, author, postA, "Aのコメント", nil)

	bad := uuid.NewString()
	for name, id := range map[string]string{"cross-post": onA.ID, "unknown": bad} {
		rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+postB+"/comments", other, map[string]any{"body": "x", "reply_to_id": id})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}

	// 削除された返信先には返信できず、既存の返信は「削除済み」になる
	reply := postComment(t, h, other, postA, "返信", &onA.ID)
	if rec := doJSON(t, h, http.MethodDelete, "/v1/comments/"+onA.ID, author, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+postA+"/comments", other, map[string]any{"body": "x", "reply_to_id": onA.ID})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("reply to deleted %d %s", rec.Code, rec.Body.String())
	}
	got := listReplyComments(t, h, other, postA)
	if r := got["返信"]; r.ID != reply.ID || !r.ReplyToRemoved || r.ReplyToName != nil {
		t.Fatalf("reply to removed %+v", r)
	}
}

func TestCommentReplyNotifications(t *testing.T) {
	h := setup(t)
	authorID, aID, bID := uuid.New(), uuid.New(), uuid.New()
	author, a, b := token(t, authorID), token(t, aID), token(t, bID)
	postID := createTextPost(t, h, author, "通知")

	top := postComment(t, h, a, postID, "最上位", nil)
	notes, _ := listNotifications(t, h, author)
	if len(notesFor(notes, postID, "comment")) != 1 {
		t.Fatalf("author should get comment: %+v", notes)
	}

	// 他人への返信: 返信先に comment_reply、投稿者に comment
	postComment(t, h, b, postID, "返信", &top.ID)
	if notes, _ = listNotifications(t, h, a); len(notesFor(notes, postID, "comment_reply")) != 1 {
		t.Fatalf("replied user should get comment_reply: %+v", notes)
	}
	if notes, _ = listNotifications(t, h, author); len(notesFor(notes, postID, "comment")) != 2 {
		t.Fatalf("author should get comment: %+v", notes)
	}

	// 投稿者宛ての返信は comment_reply のみ(二重通知しない)
	own := postComment(t, h, author, postID, "投稿者のコメント", nil)
	postComment(t, h, a, postID, "投稿者への返信", &own.ID)
	notes, _ = listNotifications(t, h, author)
	if len(notesFor(notes, postID, "comment_reply")) != 1 || len(notesFor(notes, postID, "comment")) != 2 {
		t.Fatalf("author notifications %+v", notes)
	}

	// 自分への返信は通知しない
	before, _ := listNotifications(t, h, a)
	postComment(t, h, a, postID, "自分への返信", &top.ID)
	if after, _ := listNotifications(t, h, a); len(after) != len(before) {
		t.Fatalf("self reply notified: %+v", after)
	}
}
