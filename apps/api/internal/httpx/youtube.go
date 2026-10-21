package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var youtubeClient = &http.Client{Timeout: 8 * time.Second}

type citeYouTubeReq struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

type citeYouTubeResp struct {
	Post     postDTO `json:"post"`
	Existing bool    `json:"existing"`
}

func YouTubeIDFromURL(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", "YouTubeのURLを入力してください"
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	var id string
	switch host {
	case "youtu.be":
		id = firstSegment(u.Path)
	case "youtube.com", "youtube-nocookie.com", "music.youtube.com":
		if v := u.Query().Get("v"); v != "" {
			id = v
		} else {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) >= 2 && (parts[0] == "embed" || parts[0] == "shorts" || parts[0] == "live" || parts[0] == "v") {
				id = parts[1]
			}
		}
	default:
		return "", "YouTubeのURLを入力してください"
	}
	if i := strings.IndexAny(id, "?&"); i >= 0 {
		id = id[:i]
	}
	if YouTubeVideoID(id) != "" {
		return "", "動画IDが不正です"
	}
	return id, ""
}

func firstSegment(path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	return strings.Split(path, "/")[0]
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func parseOEmbed(videoID string, raw []byte) (string, string, error) {
	var body struct {
		Title        string `json:"title"`
		ThumbnailURL string `json:"thumbnail_url"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return "", "", err
	}
	title := clipRunes(strings.TrimSpace(body.Title), 120)
	thumb := strings.TrimSpace(body.ThumbnailURL)
	if YouTubeThumbnailURL(thumb) != "" {
		thumb = "https://i.ytimg.com/vi/" + videoID + "/hqdefault.jpg"
	}
	return title, thumb, nil
}

func fetchYouTubeMeta(ctx context.Context, videoID string) (string, string, error) {
	endpoint := "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape("https://www.youtube.com/watch?v="+videoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := youtubeClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", errors.New("oembed")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", err
	}
	return parseOEmbed(videoID, raw)
}

func (s *Server) citeYouTube(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req citeYouTubeReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	fields := map[string]string{}
	videoID, msg := YouTubeIDFromURL(req.URL)
	if msg != "" {
		fields["url"] = msg
	}
	customTitle := strings.TrimSpace(req.Title)
	if e := OptionalTitle(customTitle); e != "" {
		fields["title"] = e
	}
	if e := PostBody(req.Body); e != "" {
		fields["body"] = e
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	if s.writeExistingYouTube(w, r, videoID) {
		return
	}
	sourceTitle, thumb, err := fetchYouTubeMeta(r.Context(), videoID)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "validation_error", "この動画の情報を取得できませんでした")
		return
	}
	title := customTitle
	created, err := s.q.CreatePost(r.Context(), db.CreatePostParams{
		ID:                  uuid.New(),
		AuthorID:            profile.ID,
		Title:               title,
		Body:                strings.TrimSpace(req.Body),
		Kind:                "youtube",
		YoutubeVideoID:      &videoID,
		YoutubeThumbnailUrl: &thumb,
		SourceTitle:         &sourceTitle,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && s.writeExistingYouTube(w, r, videoID) {
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

func (s *Server) writeExistingYouTube(w http.ResponseWriter, r *http.Request, videoID string) bool {
	id, err := s.q.FindYouTubePostID(r.Context(), &videoID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	if err != nil {
		MapDBError(w, err)
		return true
	}
	full, err := s.q.GetPost(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "この動画のスレはすでにあります")
		return true
	}
	if err != nil {
		MapDBError(w, err)
		return true
	}
	JSON(w, http.StatusOK, citeYouTubeResp{Post: withMine(r, s.present(postFromGet(full, false))), Existing: true})
	return true
}
