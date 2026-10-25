package httpx

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

func (s *Server) voiceConfigured(w http.ResponseWriter) bool {
	if s.voice == nil {
		voiceUnavailable(w)
		return false
	}
	return true
}
func (s *Server) voicePolicyAllowed(ctx context.Context, q *db.Queries, settings db.MeetVoiceSetting, id uuid.UUID, checkTick bool) bool {
	if !s.cfg.MeetVoiceEnabled || settings.Paused {
		return false
	}
	if checkTick && (s.cfg.MeetVoiceTickSecret == "" || settings.LastTickAt == nil || time.Since(*settings.LastTickAt) > 2*time.Minute) {
		return false
	}
	if settings.RestrictToPosts {
		allowed := false
		for _, post := range settings.AllowedPostIds {
			if post == id {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}
	used, err := q.MeetVoiceUsageMinutes(ctx)
	return err == nil && used < int64(settings.MonthlyMinutesLimit)
}
func (s *Server) voicePolicy(w http.ResponseWriter, ctx context.Context, q *db.Queries, settings db.MeetVoiceSetting, id uuid.UUID, checkTick bool) bool {
	if s.voicePolicyAllowed(ctx, q, settings, id, checkTick) {
		return true
	}
	Error(w, 503, "voice_paused", "音声通話は停止中、または利用上限に達しています。コメント・動画は引き続き利用できます")
	return false
}
func soloConfirmation(r db.MeetVoiceReservation) (bool, bool) {
	if r.SoloSince == nil {
		return false, false
	}
	since := *r.SoloSince
	if r.ConfirmedAt.After(since) {
		since = r.ConfirmedAt
	}
	elapsed := time.Since(since)
	return elapsed > 15*time.Minute, elapsed > 16*time.Minute
}

func (s *Server) releaseVoice(ctx context.Context, q *db.Queries, r db.MeetVoiceReservation) error {
	if s.voice == nil {
		return errors.New("voice provider is not configured")
	}
	if err := s.voice.Revoke(ctx, r.PostID, r.ID); err != nil {
		return err
	}
	if err := q.EndMeetVoiceParticipant(ctx, r.ID); err != nil {
		return err
	}
	return q.DeleteMeetVoiceReservation(ctx, db.DeleteMeetVoiceReservationParams{UserID: r.UserID, ID: r.ID})
}

func (s *Server) heartbeatMeetVoice(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	postID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	user, ok := s.requireProfile(w, r)
	if !ok || !s.voiceConfigured(w) {
		return
	}
	var req struct {
		ConnectionID uuid.UUID `json:"connection_id"`
		Confirm      bool      `json:"confirm"`
	}
	if err := DecodeJSON(r, &req); err != nil || req.ConnectionID == uuid.Nil {
		Error(w, 400, "bad_request", "接続を指定してください")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(context.Background())
	q := s.q.WithTx(tx)
	if err = q.LockMeetVoiceUser(ctx, user.ID); err != nil {
		MapDBError(w, err)
		return
	}
	reservation, err := q.GetMeetVoiceReservation(ctx, user.ID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (reservation.ID != req.ConnectionID || reservation.PostID != postID)) {
		Error(w, 410, "voice_ended", "通話は終了しました")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	settings, err := q.GetMeetVoiceSettings(ctx)
	if err != nil {
		MapDBError(w, err)
		return
	}
	banned, err := q.IsMeetVoiceBanned(ctx, db.IsMeetVoiceBannedParams{PostID: postID, UserID: user.ID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	people, err := s.voice.Participants(ctx, postID)
	if err != nil {
		voiceUnavailable(w)
		return
	}
	present := false
	for _, p := range people {
		if p.Identity == reservation.ID.String() {
			present = true
		}
	}
	_, postErr := q.GetPost(ctx, postID)
	if postErr != nil && !errors.Is(postErr, pgx.ErrNoRows) {
		MapDBError(w, postErr)
		return
	}
	_, expired := soloConfirmation(reservation)
	if !present || banned || postErr != nil || expired || !s.voicePolicyAllowed(ctx, q, settings, postID, true) {
		if err = s.releaseVoice(ctx, q, reservation); err != nil {
			voiceUnavailable(w)
			return
		}
		if err = tx.Commit(ctx); err != nil {
			MapDBError(w, err)
			return
		}
		Error(w, 410, "voice_ended", "通話は終了しました。参加停止・提供状況をご確認ください")
		return
	}
	if err = q.RestoreMeetVoiceParticipant(ctx, reservation.ID); err != nil {
		MapDBError(w, err)
		return
	}
	reservation, err = q.TouchMeetVoiceReservation(ctx, db.TouchMeetVoiceReservationParams{UserID: user.ID, ID: req.ConnectionID, Confirm: req.Confirm, Solo: len(people) <= 1})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		MapDBError(w, err)
		return
	}
	required, _ := soloConfirmation(reservation)
	JSON(w, 200, map[string]any{"confirmation_required": required, "participants": len(people)})
}

// Scheduler and admin stop both use the same idempotent reconciliation routine.
func (s *Server) sweepMeetVoice(ctx context.Context) (int, int, error) {
	rows, err := s.q.ListMeetVoiceReservations(ctx)
	if err != nil {
		return 0, 0, err
	}
	removed, failed := 0, 0
	for _, row := range rows {
		err := func() error {
			tx, err := s.pool.Begin(ctx)
			if err != nil {
				return err
			}
			defer tx.Rollback(context.Background())
			q := s.q.WithTx(tx)
			if err = q.LockMeetVoiceUser(ctx, row.UserID); err != nil {
				return err
			}
			current, err := q.GetMeetVoiceReservation(ctx, row.UserID)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			if current.ID != row.ID {
				return nil
			}
			settings, err := q.GetMeetVoiceSettings(ctx)
			if err != nil {
				return err
			}
			banned, err := q.IsMeetVoiceBanned(ctx, db.IsMeetVoiceBannedParams{PostID: row.PostID, UserID: row.UserID})
			if err != nil {
				return err
			}
			if s.voice == nil {
				return errors.New("voice provider unavailable")
			}
			people, err := s.voice.Participants(ctx, row.PostID)
			if err != nil {
				return err
			}
			current, err = q.SetMeetVoiceSoloState(ctx, db.SetMeetVoiceSoloStateParams{UserID: row.UserID, ID: row.ID, Solo: len(people) <= 1})
			if err != nil {
				return err
			}
			_, expired := soloConfirmation(current)
			_, postErr := q.GetPost(ctx, row.PostID)
			if postErr != nil && !errors.Is(postErr, pgx.ErrNoRows) {
				return postErr
			}
			present := false
			for _, p := range people {
				if p.Identity == row.ID.String() {
					present = true
				}
			}
			stop := banned || postErr != nil || expired || time.Since(current.LastSeenAt) > 90*time.Second || (!present && current.TokenExpiresAt.Before(time.Now())) || !s.voicePolicyAllowed(ctx, q, settings, row.PostID, false)
			if present {
				if err = q.RestoreMeetVoiceParticipant(ctx, current.ID); err != nil {
					return err
				}
			}
			if stop {
				if err = s.releaseVoice(ctx, q, current); err != nil {
					return err
				}
				if err = q.CreateMeetVoiceAction(ctx, db.CreateMeetVoiceActionParams{ID: uuid.New(), ConnectionID: &current.ID, Action: "timeout", Success: true}); err != nil {
					return err
				}
			}
			if err = tx.Commit(ctx); err != nil {
				return err
			}
			if stop {
				removed++
			}
			return nil
		}()
		if err != nil {
			failed++
		}
	}
	// Account deletion can cascade a reservation away while its bearer token
	// remains valid. Revoke such provider identities independently of profiles.
	orphans, err := s.q.ListOrphanMeetVoiceParticipants(ctx)
	if err != nil {
		return removed, failed, err
	}
	for _, orphan := range orphans {
		if s.voice == nil || s.voice.Revoke(ctx, orphan.PostID, orphan.ID) != nil {
			failed++
			continue
		}
		if err = s.q.EndMeetVoiceParticipant(ctx, orphan.ID); err != nil {
			return removed, failed, err
		}
		removed++
	}
	return removed, failed, nil
}
func (s *Server) meetVoiceTick(w http.ResponseWriter, r *http.Request) {
	if s.cfg.MeetVoiceTickSecret == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Voice-Tick-Secret")), []byte(s.cfg.MeetVoiceTickSecret)) != 1 {
		Error(w, 404, "not_found", "見つかりませんでした")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 50*time.Second)
	defer cancel()
	removed, failed, err := s.sweepMeetVoice(ctx)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if failed > 0 {
		Error(w, 503, "voice_cleanup_pending", "接続の整理が完了していません。再試行してください")
		return
	}
	if err = s.q.CleanupMeetVoiceHistory(ctx); err != nil {
		MapDBError(w, err)
		return
	}
	if err = s.q.CleanupMeetVoiceActions(ctx); err != nil {
		MapDBError(w, err)
		return
	}
	if err = s.q.MarkMeetVoiceTick(ctx); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, 200, map[string]any{"removed": removed})
}
