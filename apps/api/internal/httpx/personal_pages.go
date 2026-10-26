package httpx

import (
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"net/http"
	"strings"
	"time"
)

func personalCursor(r *http.Request, scope string) (homeCursor, *uuid.UUID, *time.Time, bool) {
	c := homeCursor{Version: 1, Scope: scope, AsOf: time.Now().UTC()}
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		var ok bool
		c, ok = decodeHomeCursor(raw, scope, time.Now())
		if !ok {
			return c, nil, nil, false
		}
		return c, &c.ID, &c.Time, true
	}
	return c, nil, nil, true
}

func pageMeta(c homeCursor, per int32, more bool) map[string]any {
	next := ""
	if more {
		next = encodeHomeCursor(c)
	}
	return map[string]any{"page": 1, "per_page": per, "total": 0, "has_more": more, "next_cursor": next}
}

func (s *Server) listSavedPostsCursor(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	_, per, _ := s.queryPage(r)
	c, id, at, ok := personalCursor(r, "saved:"+me.ID.String()+":"+strings.TrimSpace(r.URL.Query().Get("q")))
	if !ok {
		Error(w, http.StatusBadRequest, "invalid_cursor", "一覧を更新して、もう一度お試しください")
		return
	}
	keyword, codes := searchArgs(r.URL.Query().Get("q"))
	rows, err := s.q.ListSavedPostsCursor(r.Context(), db.ListSavedPostsCursorParams{UserID: me.ID, AsOf: c.AsOf, CursorID: id, CursorTime: at, Keyword: keyword, PrefectureCodes: codes, LimitN: per + 1})
	if err != nil {
		MapDBError(w, err)
		return
	}
	more := len(rows) > int(per)
	if more {
		rows = rows[:per]
		last := rows[len(rows)-1]
		c.ID, c.Time = last.ID, last.SavedAt
	}
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	liked := s.likedSet(r.Context(), ids)
	items := make([]postDTO, 0, len(rows))
	for _, row := range rows {
		item := withMine(r, s.present(postFromList(db.ListPostsNewRow{
			ID: row.ID, AuthorID: row.AuthorID, Title: row.Title, Body: row.Body, Kind: row.Kind,
			YoutubeVideoID: row.YoutubeVideoID, YoutubeThumbnailUrl: row.YoutubeThumbnailUrl, YoutubeShorts: row.YoutubeShorts,
			InstagramType: row.InstagramType, InstagramShortcode: row.InstagramShortcode, XPostID: row.XPostID,
			CloudinaryPublicID: row.CloudinaryPublicID, SourceTitle: row.SourceTitle,
			LikeCount: row.LikeCount, CommentCount: row.CommentCount, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}, liked[row.ID], true)))
		item.SavedByMe = true
		items = append(items, item)
	}
	if err := s.attachRecruitmentCovers(r.Context(), items); err != nil {
		MapDBError(w, err)
		return
	}
	if err := s.attachPracticeSummaries(r.Context(), items); err != nil {
		MapDBError(w, err)
		return
	}
	if err := s.attachInjurySummaries(r.Context(), items); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"posts": items, "meta": pageMeta(c, per, more)})
}

func (s *Server) injuryHistoryPage(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	_, per, _ := s.queryPage(r)
	c, id, at, ok := personalCursor(r, "injury-history:"+userID.String())
	if !ok {
		Error(w, http.StatusBadRequest, "invalid_cursor", "一覧を更新して、もう一度お試しください")
		return
	}
	rows, err := s.q.ListMyInjuryPostsPage(r.Context(), db.ListMyInjuryPostsPageParams{AuthorID: userID, AsOf: c.AsOf, CursorID: id, CursorTime: at, LimitN: per + 1})
	if err != nil {
		MapDBError(w, err)
		return
	}
	more := len(rows) > int(per)
	if more {
		rows = rows[:per]
		last := rows[len(rows)-1]
		c.ID, c.Time = last.ID, last.CreatedAt
	}
	ids := make([]uuid.UUID, len(rows))
	posts := make([]db.ListMyInjuryPostsRow, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
		posts[i] = db.ListMyInjuryPostsRow(row)
	}
	found, err := s.q.ListInjuryPageReports(r.Context(), db.ListInjuryPageReportsParams{AuthorID: userID, PostIds: ids})
	if err != nil {
		MapDBError(w, err)
		return
	}
	reports := make([]db.ListMyInjuryFirstReportsRow, len(found))
	for i, row := range found {
		reports[i] = db.ListMyInjuryFirstReportsRow(row)
	}
	counts, err := s.q.CountMyRepeatedInjuries(r.Context(), userID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	repeated := make([]map[string]any, 0, len(counts))
	for _, row := range counts {
		repeated = append(repeated, map[string]any{"label": injuryBodyPartLabel(row.BodyPart), "count": row.Count})
	}
	JSON(w, http.StatusOK, map[string]any{"items": buildInjuryHistory(posts, reports).Items, "repeated": repeated, "meta": pageMeta(c, per, more)})
}
