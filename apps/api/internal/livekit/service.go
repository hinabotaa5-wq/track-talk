// Package livekit isolates the voice provider. Post authorization, account-wide
// reservations and rate limits belong to the calling HTTP handler (phase 2).
package livekit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/livekit/protocol/auth"
	lk "github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/twitchtv/twirp"
)

const (
	MaxParticipants  = 4
	JoinTokenTTL     = 2 * time.Minute
	EmptyTimeout     = 60
	DepartureTimeout = 20
)

var ErrDisabled = errors.New("meet voice is disabled")
var ErrRoomConfiguration = errors.New("voice room configuration mismatch")
var ErrRoomFull = errors.New("voice room is full")

type roomService interface {
	CreateRoom(context.Context, *lk.CreateRoomRequest) (*lk.Room, error)
	ListParticipants(context.Context, *lk.ListParticipantsRequest) (*lk.ListParticipantsResponse, error)
}

type Service struct {
	rooms            roomService
	url, key, secret string
	admin            *lksdk.RoomServiceClient
	apiBase          string
	http             *http.Client
}

// JoinCredentials contains a short-lived bearer credential. Never log or persist it.
type JoinCredentials struct {
	URL           string    `json:"url"`
	Token         string    `json:"token"`
	Room          string    `json:"room"`
	ExpiresIn     int       `json:"expires_in"`
	RoomSID       string    `json:"-"`
	RoomCreatedAt time.Time `json:"-"`
}

func New(cfg config.Config) (*Service, error) {
	if !cfg.MeetVoiceEnabled {
		return nil, ErrDisabled
	}
	if err := cfg.ValidateMeetVoice(); err != nil {
		return nil, err
	}
	apiBase := strings.TrimRight(cfg.LiveKitAPIBase, "/")
	if apiBase == "" {
		apiBase = "https://" + strings.TrimPrefix(strings.TrimRight(cfg.LiveKitURL, "/"), "wss://")
	}
	admin := lksdk.NewRoomServiceClient(apiBase, cfg.LiveKitAPIKey, cfg.LiveKitAPISecret)
	return &Service{rooms: admin, admin: admin, apiBase: apiBase,
		url: cfg.LiveKitURL, key: cfg.LiveKitAPIKey, secret: cfg.LiveKitAPISecret,
		http: &http.Client{Timeout: 10 * time.Second}}, nil
}

// PrepareJoin must only be called AFTER checking access to a meet post and
// acquiring the user's cross-room reservation. It is not a public join API.
// Identity is a reservation UUID, mapped to the authenticated user in our DB.
// A new identity isolates old tokens and delayed webhooks from subsequent joins.
// The publicName must come from the user's publicly visible profile, never email.
func (s *Service) PrepareJoin(ctx context.Context, postID, identity uuid.UUID, publicName string) (JoinCredentials, error) {
	if postID == uuid.Nil || identity == uuid.Nil {
		return JoinCredentials{}, errors.New("post and connection IDs are required")
	}
	name := RoomName(postID)
	// Never issue a token first: automatic creation would lose the capacity limit.
	room, err := s.rooms.CreateRoom(ctx, &lk.CreateRoomRequest{
		Name: name, MaxParticipants: MaxParticipants,
		EmptyTimeout: EmptyTimeout, DepartureTimeout: DepartureTimeout,
	})
	if err != nil {
		return JoinCredentials{}, fmt.Errorf("create voice room: %w", err)
	}
	// CreateRoom may return an already existing room. Reject unsafe old settings.
	if room == nil || room.Name != name || room.MaxParticipants != MaxParticipants {
		return JoinCredentials{}, ErrRoomConfiguration
	}
	// Cloud may report max_participants=4 without rejecting a fifth connection.
	// The caller must still serialize reservations; check the live count again
	// immediately before signing, rather than trusting provider enforcement.
	participants, err := s.rooms.ListParticipants(ctx, &lk.ListParticipantsRequest{Room: name})
	if err != nil {
		return JoinCredentials{}, errors.New("could not verify voice room capacity")
	}
	if participants == nil {
		return JoinCredentials{}, errors.New("voice room capacity unavailable")
	}
	if len(participants.Participants) >= MaxParticipants {
		return JoinCredentials{}, ErrRoomFull
	}
	yes, no := true, false
	grant := &auth.VideoGrant{
		RoomJoin: true, Room: name,
		CanPublish: &yes, CanSubscribe: &yes, CanPublishData: &no,
		CanUpdateOwnMetadata: &no,
	}
	grant.SetCanPublishSources([]lk.TrackSource{lk.TrackSource_MICROPHONE})
	token, err := auth.NewAccessToken(s.key, s.secret).
		SetIdentity(identity.String()).SetName(publicName).
		SetValidFor(JoinTokenTTL).SetVideoGrant(grant).
		// The room can expire between issuance and connection. Preserve the cap
		// even if LiveKit recreates it from this still-valid token.
		SetRoomConfig(&lk.RoomConfiguration{
			Name: name, MaxParticipants: MaxParticipants,
			EmptyTimeout: EmptyTimeout, DepartureTimeout: DepartureTimeout,
		}).ToJWT()
	if err != nil {
		return JoinCredentials{}, errors.New("could not sign voice join token")
	}
	return JoinCredentials{URL: s.url, Token: token, Room: name, ExpiresIn: int(JoinTokenTTL.Seconds()), RoomSID: room.Sid, RoomCreatedAt: time.Unix(room.CreationTime, 0)}, nil
}

func RoomName(postID uuid.UUID) string { return "meet-voice-" + postID.String() }

func (s *Service) Participants(ctx context.Context, postID uuid.UUID) ([]*lk.ParticipantInfo, error) {
	response, err := s.admin.ListParticipants(ctx, &lk.ListParticipantsRequest{Room: RoomName(postID)})
	var providerError twirp.Error
	if errors.As(err, &providerError) && providerError.Code() == twirp.NotFound {
		return []*lk.ParticipantInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	return response.Participants, nil
}

// Mute only turns an already published microphone off. There is deliberately
// no remote-unmute operation; only the participant may enable their microphone.
func (s *Service) Mute(ctx context.Context, postID, identity uuid.UUID) error {
	people, err := s.Participants(ctx, postID)
	if err != nil {
		return err
	}
	for _, p := range people {
		if p.Identity == identity.String() {
			for _, track := range p.Tracks {
				if track.Source == lk.TrackSource_MICROPHONE {
					if _, err = s.admin.MutePublishedTrack(ctx, &lk.MuteRoomTrackRequest{Room: RoomName(postID), Identity: identity.String(), TrackSid: track.Sid, Muted: true}); err != nil {
						return err
					}
				}
			}
			return nil
		}
	}
	return errors.New("participant is not connected")
}
