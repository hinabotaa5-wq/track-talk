package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

type meetResourcesDTO struct {
	MeetKey        string `json:"meet_key"`
	OfficialURL    string `json:"official_url"`
	TimetableURL   string `json:"timetable_url"`
	YouTubeVideoID string `json:"youtube_video_id"`
	MajorSlug      string `json:"major_slug,omitempty"`
	MajorYear      int    `json:"major_year,omitempty"`
	CompetitionID  string `json:"competition_id,omitempty"`
}

func meetExternalURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true
	}
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "", false
	}
	return raw, true
}

func meetOfficialURL(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "公式サイト:"); ok {
			if link, valid := meetExternalURL(rest); valid {
				return link
			}
		}
	}
	return ""
}

func (s *Server) meetResourceMap(ctx context.Context, keys []string) (map[string]db.ListMeetResourcesRow, error) {
	rows, err := s.q.ListMeetResources(ctx, keys)
	if err != nil {
		return nil, err
	}
	result := make(map[string]db.ListMeetResourcesRow, len(rows))
	for _, row := range rows {
		result[row.MeetKey] = row
	}
	return result, nil
}

func resourcesForTarget(t meetTarget, saved db.ListMeetResourcesRow) meetResourcesDTO {
	return meetResourcesDTO{MeetKey: t.key, OfficialURL: firstNonEmpty(saved.OfficialUrl, meetOfficialURL(t.body)), TimetableURL: saved.TimetableUrl, YouTubeVideoID: saved.YoutubeVideoID, MajorSlug: t.majorSlug, MajorYear: t.majorYear, CompetitionID: t.competitionID}
}

func (s *Server) listMeetResources(w http.ResponseWriter, r *http.Request) {
	start, end := seasonRange(currentSeasonYear(s.now()))
	targets, err := s.meetTargets(r.Context(), start, end, func(_, _ time.Time) bool { return true })
	if err != nil {
		MapDBError(w, err)
		return
	}
	keys := make([]string, 0, len(targets))
	for _, target := range targets {
		keys = append(keys, target.key)
	}
	saved, err := s.meetResourceMap(r.Context(), keys)
	if err != nil {
		MapDBError(w, err)
		return
	}
	result := make([]meetResourcesDTO, 0, len(targets))
	for _, target := range targets {
		result = append(result, resourcesForTarget(target, saved[target.key]))
	}
	JSON(w, http.StatusOK, map[string]any{"meets": result})
}

func (s *Server) getPostMeetResources(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	post, ok := s.getReadablePost(w, r, id)
	if !ok {
		return
	}
	if post.Kind != "meet" {
		Error(w, http.StatusNotFound, "not_found", "実況ルームがありません")
		return
	}
	row, err := s.q.GetMeetResourcesForPost(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "実況ルームがありません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, meetResourcesDTO{MeetKey: row.MeetKey, OfficialURL: firstNonEmpty(row.OfficialUrl, meetOfficialURL(row.Body)), TimetableURL: row.TimetableUrl, YouTubeVideoID: row.YoutubeVideoID})
}

type putMeetResourcesRequest struct {
	MeetKey      string `json:"meet_key"`
	OfficialURL  string `json:"official_url"`
	TimetableURL string `json:"timetable_url"`
	YouTubeURL   string `json:"youtube_url"`
}

func (s *Server) adminPutMeetResources(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req putMeetResourcesRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "リクエストを読めませんでした")
		return
	}
	fields := map[string]string{}
	official, valid := meetExternalURL(req.OfficialURL)
	if !valid {
		fields["official_url"] = "httpまたはhttpsのURLを入力してください"
	}
	timetable, valid := meetExternalURL(req.TimetableURL)
	if !valid {
		fields["timetable_url"] = "httpまたはhttpsのURLを入力してください"
	}
	videoID := ""
	if strings.TrimSpace(req.YouTubeURL) != "" {
		var message string
		videoID, message = YouTubeIDFromURL(req.YouTubeURL)
		if message != "" {
			fields["youtube_url"] = message
		}
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	targets, err := s.adminMeetTargets(r)
	if err != nil {
		MapDBError(w, err)
		return
	}
	var target *meetTarget
	for i := range targets {
		if targets[i].key == req.MeetKey {
			target = &targets[i]
			break
		}
	}
	if target == nil {
		FieldError(w, map[string]string{"meet_key": "対象の大会ではありません"})
		return
	}
	if err = s.q.UpsertMeetResources(r.Context(), db.UpsertMeetResourcesParams{MeetKey: req.MeetKey, OfficialUrl: official, TimetableUrl: timetable, YoutubeVideoID: videoID}); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, resourcesForTarget(*target, db.ListMeetResourcesRow{MeetKey: req.MeetKey, OfficialUrl: official, TimetableUrl: timetable, YoutubeVideoID: videoID}))
}
