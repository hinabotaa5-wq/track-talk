package httpx

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

const practiceListPerPage = 24

// encodePracticeCursor は、一覧の最後の募集(日時と投稿 ID)を続きの読み込み用に包む。
func encodePracticeCursor(startsAt time.Time, id uuid.UUID) string {
	raw := startsAt.UTC().Format(time.RFC3339Nano) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodePracticeCursor(value string) (time.Time, uuid.UUID, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	at, idText, found := strings.Cut(string(raw), "|")
	if !found {
		return time.Time{}, uuid.Nil, false
	}
	startsAt, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	id, err := uuid.Parse(idText)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	return startsAt, id, true
}

// listUpcomingPractice は、練習相手の一覧(GET /v1/practice)。ログインしていなくても見られる。
// これからの、解散していない募集を日時の早い順に返す。都道府県と種目は、指定がなければすべて。
func (s *Server) listUpcomingPractice(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var prefecture, event *string
	if raw := strings.TrimSpace(q.Get("prefecture")); raw != "" {
		code, ok := parsePrefectureCode(raw)
		if !ok || code == "00" {
			Error(w, http.StatusBadRequest, "invalid_prefecture", "都道府県が正しくありません")
			return
		}
		prefecture = &code
	}
	if raw := strings.TrimSpace(q.Get("event")); raw != "" {
		if !validPracticeEvent(raw) {
			Error(w, http.StatusBadRequest, "invalid_event", "種目が正しくありません")
			return
		}
		event = &raw
	}
	var cursorTime *time.Time
	var cursorID *uuid.UUID
	if raw := q.Get("cursor"); raw != "" {
		at, id, ok := decodePracticeCursor(raw)
		if !ok {
			Error(w, http.StatusBadRequest, "invalid_cursor", "一覧を更新して、もう一度お試しください")
			return
		}
		cursorTime, cursorID = &at, &id
	}

	now := s.now()
	total, err := s.q.CountUpcomingPractice(r.Context(), db.CountUpcomingPracticeParams{Now: now, Prefecture: prefecture, Event: event})
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListUpcomingPractice(r.Context(), db.ListUpcomingPracticeParams{
		Now:        now,
		Prefecture: prefecture,
		Event:      event,
		CursorTime: cursorTime,
		CursorID:   cursorID,
		LimitN:     practiceListPerPage + 1,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	next := ""
	if len(rows) > practiceListPerPage {
		rows = rows[:practiceListPerPage]
		last := rows[len(rows)-1]
		next = encodePracticeCursor(last.StartsAt, last.ID)
	}

	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	liked := s.likedSet(r.Context(), ids)
	saved := s.savedSet(r.Context(), ids)
	items := make([]postDTO, 0, len(rows))
	for _, row := range rows {
		listRow := db.ListPostsNewRow{
			ID:                  row.ID,
			AuthorID:            row.AuthorID,
			Title:               row.Title,
			Body:                row.Body,
			Kind:                row.Kind,
			YoutubeVideoID:      row.YoutubeVideoID,
			YoutubeThumbnailUrl: row.YoutubeThumbnailUrl,
			YoutubeShorts:       row.YoutubeShorts,
			InstagramType:       row.InstagramType,
			InstagramShortcode:  row.InstagramShortcode,
			XPostID:             row.XPostID,
			CloudinaryPublicID:  row.CloudinaryPublicID,
			SourceTitle:         row.SourceTitle,
			LikeCount:           row.LikeCount,
			CommentCount:        row.CommentCount,
			CreatedAt:           row.CreatedAt,
			UpdatedAt:           row.UpdatedAt,
		}
		item := withMine(r, s.present(postFromList(listRow, liked[row.ID], true)))
		item.SavedByMe = saved[row.ID]
		items = append(items, item)
	}
	if err := s.attachPracticeSummaries(r.Context(), items); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"posts": items, "total": total, "next_cursor": next})
}
