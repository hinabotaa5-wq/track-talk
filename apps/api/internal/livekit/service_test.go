package livekit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	lk "github.com/livekit/protocol/livekit"
)

type fakeRooms struct {
	req          *lk.CreateRoomRequest
	err          error
	wrongLimit   bool
	participants []*lk.ParticipantInfo
	listErr      error
}

func (f *fakeRooms) ListParticipants(context.Context, *lk.ListParticipantsRequest) (*lk.ListParticipantsResponse, error) {
	return &lk.ListParticipantsResponse{Participants: f.participants}, f.listErr
}

func TestPrepareJoinChecksLiveCapacity(t *testing.T) {
	for _, tc := range []struct {
		name    string
		count   int
		listErr error
		want    error
	}{
		{name: "four connected", count: 4, want: ErrRoomFull},
		{name: "over capacity", count: 5, want: ErrRoomFull},
		{name: "provider unavailable", listErr: errors.New("provider unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rooms := &fakeRooms{participants: make([]*lk.ParticipantInfo, tc.count), listErr: tc.listErr}
			s := &Service{rooms: rooms, url: "wss://test.livekit.cloud", key: "test-key", secret: "test-secret"}
			credentials, err := s.PrepareJoin(context.Background(), uuid.New(), uuid.New(), "公開名")
			if err == nil || credentials.Token != "" || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatal("unsafe token issued or wrong capacity error")
			}
		})
	}
}

func (f *fakeRooms) CreateRoom(_ context.Context, req *lk.CreateRoomRequest) (*lk.Room, error) {
	f.req = req
	if f.err != nil {
		return nil, f.err
	}
	limit := req.MaxParticipants
	if f.wrongLimit {
		limit = 0
	}
	return &lk.Room{Name: req.Name, MaxParticipants: limit}, nil
}

func TestPrepareJoinPermissions(t *testing.T) {
	rooms := &fakeRooms{}
	s := &Service{rooms: rooms, url: "wss://test.livekit.cloud", key: "test-key", secret: "test-secret"}
	post, user := uuid.New(), uuid.New()
	start := time.Now()
	c, err := s.PrepareJoin(context.Background(), post, user, "公開名")
	if err != nil {
		t.Fatal(err)
	}
	if rooms.req.MaxParticipants != 4 || rooms.req.EmptyTimeout != 60 || rooms.req.DepartureTimeout != 20 {
		t.Fatalf("unsafe room: %v", rooms.req)
	}
	if c.ExpiresIn != 120 || c.Room != "meet-voice-"+post.String() {
		t.Fatal("invalid join response")
	}
	parsed, err := jwt.Parse(c.Token, func(*jwt.Token) (any, error) { return []byte("test-secret"), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("test-key"))
	if err != nil {
		t.Fatal("invalid token signature or claims")
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if claims["sub"] != user.String() || claims["name"] != "公開名" {
		t.Fatal("wrong participant identity")
	}
	expiry, _ := claims.GetExpirationTime()
	if expiry.Time.Before(start.Add(JoinTokenTTL-time.Second)) || expiry.Time.After(time.Now().Add(JoinTokenTTL)) {
		t.Fatal("wrong token lifetime")
	}
	v := claims["video"].(map[string]any)
	roomConfig := claims["roomConfig"].(map[string]any)
	if roomConfig["name"] != c.Room || roomConfig["maxParticipants"] != float64(4) {
		t.Fatal("token could recreate a room without the capacity limit")
	}
	if v["room"] != c.Room || v["roomJoin"] != true || v["canPublish"] != true || v["canSubscribe"] != true || v["canPublishData"] != false || v["canUpdateOwnMetadata"] != false {
		t.Fatal("wrong room permissions")
	}
	sources := v["canPublishSources"].([]any)
	if len(sources) != 1 || sources[0] != "microphone" {
		t.Fatal("publication is not microphone-only")
	}
	for _, p := range []string{"roomAdmin", "roomCreate", "roomList", "roomRecord", "ingressAdmin"} {
		if v[p] == true {
			t.Fatalf("unexpected %s", p)
		}
	}
}

func TestPrepareJoinFailsClosed(t *testing.T) {
	for _, rooms := range []*fakeRooms{{err: errors.New("unavailable")}, {wrongLimit: true}} {
		s := &Service{rooms: rooms, key: "key", secret: "secret"}
		c, err := s.PrepareJoin(context.Background(), uuid.New(), uuid.New(), "")
		if err == nil || c.Token != "" {
			t.Fatal("issued token without a safe room")
		}
	}
	rooms := &fakeRooms{}
	s := &Service{rooms: rooms}
	if _, err := s.PrepareJoin(context.Background(), uuid.Nil, uuid.New(), ""); err == nil || rooms.req != nil {
		t.Fatal("accepted empty ID")
	}
}

func TestNewDisabled(t *testing.T) {
	if _, err := New(config.Config{}); !errors.Is(err, ErrDisabled) {
		t.Fatal(err)
	}
	if _, err := New(config.Config{MeetVoiceEnabled: true}); err == nil {
		t.Fatal("accepted missing configuration")
	}
}
