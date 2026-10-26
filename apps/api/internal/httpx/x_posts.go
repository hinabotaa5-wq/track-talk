package httpx

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var xPostPath = regexp.MustCompile(`^/(?:[A-Za-z0-9_]{1,15}/status|i/web/status|i/status)/([0-9]{1,25})(?:/(?:photo|video)/[1-4])?/?$`)

func XPostFromURL(raw string) (string, string) {
	const msg = "Xの投稿のURLを入力してください"
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Port() != "" {
		return "", msg
	}
	switch strings.ToLower(u.Hostname()) {
	case "x.com", "www.x.com", "mobile.x.com", "twitter.com", "www.twitter.com", "mobile.twitter.com":
	default:
		return "", msg
	}
	match := xPostPath.FindStringSubmatch(u.Path)
	if match == nil {
		return "", msg
	}
	return match[1], ""
}

func (s *Server) citeXPost(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req citeInstagramReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	fields := map[string]string{}
	code, msg := XPostFromURL(req.URL)
	if msg != "" {
		fields["url"] = msg
	}
	title := strings.TrimSpace(req.Title)
	if e := OptionalTitle(title); e != "" {
		fields["title"] = e
	}
	if e := PostBody(req.Body); e != "" {
		fields["body"] = e
	}
	var cover *string
	if id := strings.TrimSpace(req.ImagePublicID); id != "" {
		if e := ImagePublicID(id); e != "" {
			fields["image_public_id"] = e
		}
		cover = &id
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	// 同じ投稿のスレッドが既にあれば、そちらを返す(付けた表紙は使わない)。
	if s.writeExistingXPost(w, r, code) {
		return
	}
	if cover != nil {
		if !s.uploadIssuedToMe(w, r, profile.ID, *cover, "image_public_id", "サムネの画像") {
			return
		}
		if err := s.cloud.ImageReady(r.Context(), *cover); err != nil {
			s.purgeRejectedUpload(err, "image", *cover)
			writeAssetReadyError(w, err, "サムネの画像")
			return
		}
	}
	created, err := s.q.CreatePost(r.Context(), db.CreatePostParams{
		ID:                 uuid.New(),
		AuthorID:           profile.ID,
		Title:              title,
		Body:               strings.TrimSpace(req.Body),
		Kind:               "x",
		XPostID:            &code,
		CloudinaryPublicID: cover,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && s.writeExistingXPost(w, r, code) {
			return
		}
		MapDBError(w, err)
		return
	}
	full, err := s.q.GetPost(r.Context(), created.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusCreated, citeYouTubeResp{Post: withMine(r, s.present(postFromGet(full, false))), Existing: false})
}

func (s *Server) writeExistingXPost(w http.ResponseWriter, r *http.Request, code string) bool {
	id, err := s.q.FindXPostID(r.Context(), &code)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	if err != nil {
		MapDBError(w, err)
		return true
	}
	full, err := s.q.GetPost(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "この投稿のスレはすでにあります")
		return true
	}
	if err != nil {
		MapDBError(w, err)
		return true
	}
	JSON(w, http.StatusOK, citeYouTubeResp{Post: withMine(r, s.present(postFromGet(full, false))), Existing: true})
	return true
}
