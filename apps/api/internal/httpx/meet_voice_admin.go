package httpx

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

func (s *Server) adminMeetVoice(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	settings, err := s.q.GetMeetVoiceSettings(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListMeetVoiceReservations(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	bans, err := s.q.ListMeetVoiceBans(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	used, err := s.q.MeetVoiceUsageMinutes(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	if rows == nil {
		rows = []db.ListMeetVoiceReservationsRow{}
	}
	if bans == nil {
		bans = []db.ListMeetVoiceBansRow{}
	}
	JSON(w, 200, map[string]any{"configured": s.voice != nil, "enabled": s.cfg.MeetVoiceEnabled, "settings": settings, "connections": rows, "bans": bans, "estimated_minutes": used})
}
func (s *Server) adminConfigureMeetVoice(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req db.UpdateMeetVoiceSettingsParams
	if err := DecodeJSON(r, &req); err != nil || req.MonthlyMinutesLimit < 1 || req.MonthlyMinutesLimit > 100000 || len(req.AllowedPostIds) > 100 {
		Error(w, 400, "bad_request", "設定内容を確認してください")
		return
	}
	if req.AllowedPostIds == nil {
		req.AllowedPostIds = []uuid.UUID{}
	}
	// Update takes an exclusive settings-row lock. In-flight joins hold SHARE,
	// so no token can slip through once pausing has committed.
	if _, err := s.q.UpdateMeetVoiceSettings(r.Context(), req); err != nil {
		MapDBError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 50*time.Second)
	defer cancel()
	_, failed, err := s.sweepMeetVoice(ctx)
	success := err == nil && failed == 0
	action := "configure"
	if req.Paused {
		action = "pause"
	}
	if auditErr := s.q.CreateMeetVoiceAction(ctx, db.CreateMeetVoiceActionParams{ID: uuid.New(), ActorID: &admin.ID, Action: action, Success: success}); auditErr != nil {
		MapDBError(w, auditErr)
		return
	}
	if !success {
		Error(w, 503, "voice_cleanup_pending", "設定は保存しました。一部の退出処理が未完了です。同じ操作を再試行してください")
		return
	}
	s.adminMeetVoice(w, r)
}
func (s *Server) adminModerateMeetVoice(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok || !s.voiceConfigured(w) {
		return
	}
	id, ok := parseUUIDParam(w, r, "connectionID")
	if !ok {
		return
	}
	var req struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := DecodeJSON(r, &req); err != nil || (req.Action != "mute" && req.Action != "remove" && req.Action != "ban") || len([]rune(req.Reason)) > 500 {
		Error(w, 400, "bad_request", "操作を確認してください")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	member, err := s.q.GetMeetVoiceParticipant(ctx, id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if member.UserID == nil {
		Error(w, 404, "not_found", "対象は退会済みです")
		return
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(context.Background())
	q := s.q.WithTx(tx)
	if err = q.LockMeetVoiceUser(ctx, *member.UserID); err != nil {
		MapDBError(w, err)
		return
	}
	if req.Action == "ban" {
		if err = q.UpsertMeetVoiceBan(ctx, db.UpsertMeetVoiceBanParams{PostID: member.PostID, UserID: *member.UserID, Reason: strings.TrimSpace(req.Reason)}); err != nil {
			MapDBError(w, err)
			return
		}
	}
	current, err := q.GetMeetVoiceReservation(ctx, *member.UserID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		MapDBError(w, err)
		return
	}
	success := true
	if err == nil && current.PostID == member.PostID && (current.ID == id || req.Action == "ban") {
		if req.Action == "mute" {
			success = s.voice.Mute(ctx, current.PostID, current.ID) == nil
		} else {
			success = s.releaseVoice(ctx, q, current) == nil
		}
	} else if req.Action != "ban" {
		Error(w, 409, "voice_ended", "その接続は終了しています")
		return
	}
	if err = q.CreateMeetVoiceAction(ctx, db.CreateMeetVoiceActionParams{ID: uuid.New(), ActorID: &admin.ID, ConnectionID: &id, TargetUserID: member.UserID, PostID: &member.PostID, Action: req.Action, Success: success}); err != nil {
		MapDBError(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		MapDBError(w, err)
		return
	}
	if !success {
		Error(w, 503, "voice_cleanup_pending", "操作を記録しました。接続の操作が未完了です。再試行してください")
		return
	}
	w.WriteHeader(204)
}
func (s *Server) adminUnbanMeetVoice(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	userID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		PostID uuid.UUID `json:"post_id"`
	}
	if err := DecodeJSON(r, &req); err != nil || req.PostID == uuid.Nil {
		Error(w, 400, "bad_request", "実況ルームを指定してください")
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(context.Background())
	q := s.q.WithTx(tx)
	if err = q.LockMeetVoiceUser(r.Context(), userID); err != nil {
		MapDBError(w, err)
		return
	}
	if err = q.DeleteMeetVoiceBan(r.Context(), db.DeleteMeetVoiceBanParams{PostID: req.PostID, UserID: userID}); err != nil {
		MapDBError(w, err)
		return
	}
	if err = q.CreateMeetVoiceAction(r.Context(), db.CreateMeetVoiceActionParams{ID: uuid.New(), ActorID: &admin.ID, TargetUserID: &userID, PostID: &req.PostID, Action: "unban", Success: true}); err != nil {
		MapDBError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	w.WriteHeader(204)
}
