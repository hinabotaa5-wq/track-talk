package httpx

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

func insertNotification(ctx context.Context, q *db.Queries, recipient uuid.UUID, kind string, postID uuid.UUID, commentID, applicationID *uuid.UUID) error {
	return q.InsertNotification(ctx, db.InsertNotificationParams{
		ID:            uuid.New(),
		RecipientID:   recipient,
		Kind:          kind,
		PostID:        &postID,
		CommentID:     commentID,
		ApplicationID: applicationID,
	})
}

func notificationJSON(row db.ListMyNotificationsRow) map[string]any {
	return map[string]any{
		"id":                row.ID,
		"kind":              row.Kind,
		"post_id":           row.PostID,
		"post_title":        row.PostTitle,
		"review_request_id": row.ReviewRequestID,
		"read":              row.ReadAt != nil,
		"created_at":        row.CreatedAt,
	}
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	page, per, offset := s.queryPage(r)
	total, err := s.q.CountMyNotifications(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	unread, err := s.q.CountUnreadNotifications(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListMyNotifications(r.Context(), db.ListMyNotificationsParams{
		RecipientID: p.ID,
		Limit:       per,
		Offset:      offset,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, notificationJSON(row))
	}
	JSON(w, http.StatusOK, map[string]any{
		"notifications": out,
		"unread_count":  unread,
		"meta":          meta{Page: int(page), PerPage: int(per), Total: total},
	})
}

func (s *Server) markNotificationRead(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.MarkNotificationRead(r.Context(), db.MarkNotificationReadParams{
		ID:          id,
		RecipientID: p.ID,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) markAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	if err := s.q.MarkAllNotificationsRead(r.Context(), p.ID); err != nil {
		MapDBError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
