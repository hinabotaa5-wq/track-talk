package httpx

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

// 投稿の作成を冪等にする(#382)。
//
// Web は投稿の意図ごとに request_key(UUID)を1つ作り、再試行でも同じ値を送る。
// posts には (author_id, request_key) の一意インデックスがあり、同じキーの2件目は作られない。
// 同時に届いた2件は INSERT の一意制約で片方だけが残り、負けた側は先に作られた投稿を返す。
const (
	postRequestKeyIndex = "posts_author_request_key_idx"
	postMediaIndex      = "posts_cloudinary_public_id_active_idx"
)

// replayPost は、同じ request_key で既に作った投稿があれば、それを 200 で返して true を返す。
// 動画・写真の準備確認や通知は、最初の1回で済んでいるのでやり直さない。
func (s *Server) replayPost(w http.ResponseWriter, r *http.Request, author uuid.UUID, key *uuid.UUID) bool {
	if key == nil {
		return false
	}
	row, err := s.q.GetPostByRequestKey(r.Context(), db.GetPostByRequestKeyParams{AuthorID: author, RequestKey: key})
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	if err != nil {
		MapDBError(w, err)
		return true
	}
	if row.DeletedAt != nil {
		Error(w, http.StatusConflict, "conflict", "この投稿はすでに削除されています")
		return true
	}
	s.writePost(w, r, row.ID, http.StatusOK)
	return true
}

// writeCreatePostError は投稿の INSERT の失敗を返す。
// 同じキーの同時リクエストに負けた場合は、先に作られた投稿を返す。
// 同じ写真・動画が別の投稿に使われている場合は、項目のエラーにする。
func (s *Server) writeCreatePostError(w http.ResponseWriter, r *http.Request, err error, params db.CreatePostParams) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
		(pgErr.ConstraintName == postRequestKeyIndex || pgErr.ConstraintName == postMediaIndex) {
		// 同時に届いた同じキーの相手は、ここではもうコミット済み(一意制約の待ちが解けた後)。
		// どちらの一意制約に先に当たったかは決まらないので、キーがあればまず探す。
		if s.replayPost(w, r, params.AuthorID, params.RequestKey) {
			return
		}
		if pgErr.ConstraintName == postMediaIndex {
			writeMediaAlreadyPosted(w, params.Kind)
			return
		}
	}
	MapDBError(w, err)
}

func writeMediaAlreadyPosted(w http.ResponseWriter, kind string) {
	if kind == "video" {
		FieldError(w, map[string]string{"video_public_id": "この動画はすでに投稿されています"})
		return
	}
	FieldError(w, map[string]string{"image_public_id": "この写真はすでに投稿されています"})
}
