package httpx

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type citeInstagramReq struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// InstagramFromURL は公開の投稿(/p/)とリール(/reel/)の URL から種類とショートコードを取り出す。
// instagram.com 以外のドメインや、それ以外のページ(プロフィール・ストーリーズなど)は受け付けない。
func InstagramFromURL(raw string) (string, string, string) {
	const msg = "Instagramの投稿かリールのURLを入力してください"
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Port() != "" {
		return "", "", msg
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	switch host {
	case "instagram.com", "www.instagram.com", "m.instagram.com":
	default:
		return "", "", msg
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// https://www.instagram.com/<ユーザー名>/p/<コード>/ の形もある
	if len(parts) >= 3 && (parts[1] == "p" || parts[1] == "reel" || parts[1] == "reels") {
		parts = parts[1:]
	}
	if len(parts) < 2 {
		return "", "", msg
	}
	var kind string
	switch parts[0] {
	case "p":
		kind = "p"
	case "reel", "reels":
		kind = "reel"
	default:
		return "", "", msg
	}
	code := parts[1]
	if e := InstagramShortcode(code); e != "" {
		return "", "", e
	}
	return kind, code, ""
}

func InstagramShortcode(v string) string {
	if len(v) < 5 || len(v) > 64 {
		return "投稿のURLが不正です"
	}
	for _, r := range v {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' && r != '-' {
			return "投稿のURLが不正です"
		}
	}
	return ""
}

func (s *Server) citeInstagram(w http.ResponseWriter, r *http.Request) {
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
	kind, code, msg := InstagramFromURL(req.URL)
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
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	if s.writeExistingInstagram(w, r, code) {
		return
	}
	created, err := s.q.CreatePost(r.Context(), db.CreatePostParams{
		ID:                 uuid.New(),
		AuthorID:           profile.ID,
		Title:              title,
		Body:               strings.TrimSpace(req.Body),
		Kind:               "instagram",
		InstagramType:      &kind,
		InstagramShortcode: &code,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && s.writeExistingInstagram(w, r, code) {
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

func (s *Server) writeExistingInstagram(w http.ResponseWriter, r *http.Request, code string) bool {
	id, err := s.q.FindInstagramPostID(r.Context(), &code)
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
