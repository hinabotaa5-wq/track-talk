package httpx

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	voice "github.com/hinata/track-talk/apps/api/internal/livekit"
	"github.com/jackc/pgx/v5"
)

// Participant counts come from LiveKit; unexpired pending reservations also
// reserve seats while browsers establish their connection.
func (s *Server) voiceTarget(w http.ResponseWriter, r *http.Request) (uuid.UUID, db.Profile, bool) {
	w.Header().Set("Cache-Control", "no-store")
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return uuid.Nil, db.Profile{}, false
	}
	profile, ok := s.requireProfile(w, r)
	if !ok {
		return uuid.Nil, db.Profile{}, false
	}
	post, ok := s.getReadablePost(w, r, id)
	if !ok {
		return uuid.Nil, db.Profile{}, false
	}
	if post.Kind != "meet" {
		Error(w, 404, "not_found", "実況ルームがありません")
		return uuid.Nil, db.Profile{}, false
	}
	return id, profile, true
}

func (s *Server) voiceAvailable(w http.ResponseWriter) bool {
	if !s.cfg.MeetVoiceEnabled {
		Error(w, 404, "not_found", "音声通話は提供していません")
		return false
	}
	if s.voice == nil {
		voiceUnavailable(w)
		return false
	}
	return true
}

func voiceUnavailable(w http.ResponseWriter) {
	Error(w, http.StatusServiceUnavailable, "voice_unavailable", "音声通話に接続できません。しばらくしてからお試しください")
}

func (s *Server) getMeetVoice(w http.ResponseWriter, r *http.Request) {
	id, _, ok := s.voiceTarget(w, r)
	if !ok {
		return
	}
	if !s.cfg.MeetVoiceEnabled {
		JSON(w, 200, map[string]any{"enabled": false, "participants": 0, "capacity": voice.MaxParticipants, "full": false})
		return
	}
	if !s.voiceAvailable(w) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	settings, err := s.q.GetMeetVoiceSettings(ctx)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if !s.voicePolicyAllowed(ctx, s.q, settings, id, true) {
		JSON(w, 200, map[string]any{"enabled": false, "participants": 0, "capacity": voice.MaxParticipants, "full": false})
		return
	}
	participants, err := s.voice.Participants(ctx, id)
	if err != nil {
		voiceUnavailable(w)
		return
	}
	JSON(w, 200, map[string]any{"enabled": true, "participants": len(participants), "capacity": voice.MaxParticipants, "full": len(participants) >= voice.MaxParticipants})
}

func (s *Server) joinMeetVoice(w http.ResponseWriter, r *http.Request) {
	id, profile, ok := s.voiceTarget(w, r)
	if !ok || !s.voiceAvailable(w) {
		return
	}
	attempts, err := s.q.ConsumeMeetVoiceJoinAttempt(r.Context(), profile.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if attempts > 5 {
		w.Header().Set("Retry-After", "60")
		Error(w, 429, "voice_rate_limited", "参加操作が多すぎます。1分ほど待ってお試しください")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(context.Background())
	q := s.q.WithTx(tx)
	// Always acquire account then room, including leave, to avoid lock inversion.
	if err = q.LockMeetVoiceUser(ctx, profile.ID); err != nil {
		MapDBError(w, err)
		return
	}
	settings, err := q.LockMeetVoiceSettings(ctx)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if !s.voicePolicy(w, ctx, q, settings, id, true) {
		return
	}
	count, err := q.CountMeetVoiceReservations(ctx)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if count >= 1000 {
		voiceUnavailable(w)
		return
	}
	banned, err := q.IsMeetVoiceBanned(ctx, db.IsMeetVoiceBannedParams{PostID: id, UserID: profile.ID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if banned {
		Error(w, 403, "voice_banned", "この実況ルームの音声通話には参加できません")
		return
	}
	previous, err := q.GetMeetVoiceReservation(ctx, profile.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		MapDBError(w, err)
		return
	}
	if err == nil {
		live, providerErr := s.voice.Participants(ctx, previous.PostID)
		if providerErr != nil {
			voiceUnavailable(w)
			return
		}
		connected := false
		for _, p := range live {
			if p.Identity == previous.ID.String() {
				connected = true
				break
			}
		}
		if connected || (previous.State != "disconnected" && previous.TokenExpiresAt.After(time.Now())) {
			Error(w, 409, "voice_already_joined", "別の端末・タブ、または別の実況ルームで参加中です。先に退出してください")
			return
		}
		// Reconcile missing/late webhooks and revoke even expired initial tokens:
		// LiveKit may have refreshed a token while the participant was connected.
		if err = s.voice.Revoke(ctx, previous.PostID, previous.ID); err != nil {
			voiceUnavailable(w)
			return
		}
		if err = q.EndMeetVoiceParticipant(ctx, previous.ID); err != nil {
			MapDBError(w, err)
			return
		}
		if err = q.DeleteMeetVoiceReservation(ctx, db.DeleteMeetVoiceReservationParams{UserID: profile.ID, ID: previous.ID}); err != nil {
			MapDBError(w, err)
			return
		}
	}
	if err = q.LockMeetVoiceRoom(ctx, id); err != nil {
		MapDBError(w, err)
		return
	}
	if _, err = q.LockReadableMeetVoicePost(ctx, id); err != nil {
		MapDBError(w, err)
		return
	}
	participants, err := s.voice.Participants(ctx, id)
	if err != nil {
		voiceUnavailable(w)
		return
	}
	occupied := map[string]bool{}
	for _, p := range participants {
		occupied[p.Identity] = true
	}
	pending, err := q.ListPendingMeetVoiceReservations(ctx, id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	for _, identity := range pending {
		occupied[identity.String()] = true
	}
	if len(occupied) >= voice.MaxParticipants {
		Error(w, 409, "voice_full", "音声通話は満員です（最大4人）")
		return
	}
	identity := uuid.New()
	publicName := "匿名"
	if profile.NicknamePublic && profile.Nickname != nil {
		publicName = *profile.Nickname
	}
	credentials, err := s.voice.PrepareJoin(ctx, id, identity, publicName)
	if errors.Is(err, voice.ErrRoomFull) {
		Error(w, 409, "voice_full", "音声通話は満員です（最大4人）")
		return
	}
	if err != nil || credentials.RoomSID == "" {
		voiceUnavailable(w)
		return
	}
	err = q.CreateMeetVoiceReservation(ctx, db.CreateMeetVoiceReservationParams{UserID: profile.ID, ID: identity, PostID: id, TokenExpiresAt: time.Now().Add(voice.JoinTokenTTL + 10*time.Second)})
	if err != nil {
		MapDBError(w, err)
		return
	}
	err = q.UpsertMeetVoiceSession(ctx, db.UpsertMeetVoiceSessionParams{RoomSid: credentials.RoomSID, PostID: id, RoomName: credentials.Room, StartedAt: credentials.RoomCreatedAt})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err = q.InsertMeetVoiceParticipant(ctx, db.InsertMeetVoiceParticipantParams{ID: identity, UserID: &profile.ID, PostID: id, RoomSid: credentials.RoomSID, PublicName: publicName}); err != nil {
		MapDBError(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, 200, struct {
		voice.JoinCredentials
		ConnectionID uuid.UUID `json:"connection_id"`
	}{credentials, identity})
}

func (s *Server) leaveMeetVoice(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	profile, ok := s.requireProfile(w, r)
	if !ok || !s.voiceConfigured(w) {
		return
	}
	var req struct {
		ConnectionID uuid.UUID `json:"connection_id"`
	}
	if err := DecodeJSON(r, &req); err != nil || req.ConnectionID == uuid.Nil {
		Error(w, 400, "bad_request", "退出する接続を指定してください")
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
	if err = q.LockMeetVoiceUser(ctx, profile.ID); err != nil {
		MapDBError(w, err)
		return
	}
	previous, err := q.GetMeetVoiceReservation(ctx, profile.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		w.WriteHeader(204)
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	// A delayed cleanup from an old page must never remove the new connection.
	if previous.ID != req.ConnectionID || previous.PostID != id {
		w.WriteHeader(204)
		return
	}
	if err = s.voice.Revoke(ctx, id, previous.ID); err != nil {
		voiceUnavailable(w)
		return
	}
	if err = q.EndMeetVoiceParticipant(ctx, previous.ID); err != nil {
		MapDBError(w, err)
		return
	}
	if err = q.DeleteMeetVoiceReservation(ctx, db.DeleteMeetVoiceReservationParams{UserID: profile.ID, ID: previous.ID}); err != nil {
		MapDBError(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		MapDBError(w, err)
		return
	}
	w.WriteHeader(204)
}
