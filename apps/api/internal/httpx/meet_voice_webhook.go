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
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/webhook"
)

func (s *Server) meetVoiceWebhook(w http.ResponseWriter, r *http.Request) {
	if !s.voiceConfigured(w) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	event, err := webhook.ReceiveWebhookEvent(r, auth.NewSimpleKeyProvider(s.cfg.LiveKitAPIKey, s.cfg.LiveKitAPISecret))
	if err != nil {
		Error(w, 401, "invalid_webhook", "通知の署名を確認できませんでした")
		return
	}
	switch event.Event {
	case "room_started", "room_finished", "participant_joined", "participant_left", "participant_connection_aborted":
	default:
		w.WriteHeader(204)
		return
	}
	if event.Room == nil || !strings.HasPrefix(event.Room.Name, "meet-voice-") {
		w.WriteHeader(204)
		return
	}
	postID, err := uuid.Parse(strings.TrimPrefix(event.Room.Name, "meet-voice-"))
	if err != nil {
		Error(w, 400, "bad_request", "通知のルームが不正です")
		return
	}
	eventID, err := uuid.Parse(event.Id)
	if err != nil || eventID == uuid.Nil || event.Room.Sid == "" || event.CreatedAt <= 0 || event.CreatedAt > time.Now().Add(time.Minute).Unix() {
		Error(w, 400, "bad_request", "通知の形式が不正です")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(context.Background())
	q := s.q.WithTx(tx)
	created := time.Unix(event.CreatedAt, 0)
	_, err = q.RecordMeetVoiceWebhook(ctx, db.RecordMeetVoiceWebhookParams{ID: eventID, Event: event.Event, RoomSid: event.Room.Sid, CreatedAt: created})
	if errors.Is(err, pgx.ErrNoRows) {
		w.WriteHeader(204)
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	exists, err := q.MeetVoicePostExists(ctx, postID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if exists {
		// SID, not room name, separates historical rooms from a recreated room.
		started := created
		if event.Room.CreationTime > 0 {
			started = time.Unix(event.Room.CreationTime, 0)
		}
		if err = q.UpsertMeetVoiceSession(ctx, db.UpsertMeetVoiceSessionParams{RoomSid: event.Room.Sid, PostID: postID, RoomName: event.Room.Name, StartedAt: started}); err != nil {
			MapDBError(w, err)
			return
		}
		switch event.Event {
		case "room_finished":
			err = q.FinishMeetVoiceSession(ctx, db.FinishMeetVoiceSessionParams{RoomSid: event.Room.Sid, EndedAt: &created})
			if err == nil {
				err = q.EndMeetVoiceRoomParticipants(ctx, event.Room.Sid)
			}
		case "participant_joined", "participant_left", "participant_connection_aborted":
			if event.Participant != nil {
				identity, parseErr := uuid.Parse(event.Participant.Identity)
				if parseErr == nil && event.Participant.Sid != "" {
					if event.Event == "participant_joined" {
						err = q.UpdateMeetVoiceParticipantJoined(ctx, db.UpdateMeetVoiceParticipantJoinedParams{ID: identity, PostID: postID, ParticipantSid: event.Participant.Sid, LastEventAt: &created})
					} else {
						err = q.UpdateMeetVoiceParticipantLeft(ctx, db.UpdateMeetVoiceParticipantLeftParams{ID: identity, PostID: postID, ParticipantSid: event.Participant.Sid, LastEventAt: &created})
						if err == nil {
							err = q.EndMeetVoiceWebhookParticipant(ctx, db.EndMeetVoiceWebhookParticipantParams{ID: identity, LastEventAt: &created})
						}
					}
				}
			}
		}
		if err != nil {
			MapDBError(w, err)
			return
		}
	}
	if err = q.PruneMeetVoiceWebhookEvents(ctx); err != nil {
		MapDBError(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		MapDBError(w, err)
		return
	}
	w.WriteHeader(204)
}
