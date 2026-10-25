package livekit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/livekit/protocol/auth"
)

// Revoke uses the documented Cloud API extension unavailable in the Go 1.25
// compatible SDK. The explicit cutoff also revokes pending/refreshed tokens,
// including a participant who has already left. New joins use a new identity.
func (s *Service) Revoke(ctx context.Context, postID, identity uuid.UUID) error {
	token, err := auth.NewAccessToken(s.key, s.secret).SetValidFor(time.Minute).
		SetVideoGrant(&auth.VideoGrant{RoomAdmin: true, Room: RoomName(postID)}).ToJWT()
	if err != nil {
		return errors.New("could not sign voice management request")
	}
	body, err := json.Marshal(struct {
		Room          string `json:"room"`
		Identity      string `json:"identity"`
		RevokeTokenTS int64  `json:"revoke_token_ts"`
	}{RoomName(postID), identity.String(), time.Now().Unix() + 1})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiBase+"/twirp/livekit.RoomService/RemoveParticipant", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	response, err := s.http.Do(req)
	if err != nil {
		return errors.New("voice revocation request failed")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	// Do not treat NotFound as success: callers may only free reservations after
	// confirmed strict revocation, otherwise a cached token could still reconnect.
	if response.StatusCode != http.StatusOK {
		return errors.New("voice revocation was not confirmed")
	}
	return nil
}
