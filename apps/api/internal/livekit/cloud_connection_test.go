package livekit

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/joho/godotenv"
	lk "github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

// Explicit opt-in only: uses four synthetic participants in one temporary
// development room. Silent Opus packets verify transport, not audible quality
// or physical devices. No microphone, recording, or stored join tokens.
func TestCloudFourParticipants(t *testing.T) {
	if os.Getenv("LIVEKIT_TEST") != "1" {
		t.Skip("set LIVEKIT_TEST=1 to check the development LiveKit project")
	}
	_ = godotenv.Load("../../../../.env")
	cfg := config.Config{MeetVoiceEnabled: true, LiveKitURL: os.Getenv("LIVEKIT_URL"), LiveKitAPIKey: os.Getenv("LIVEKIT_API_KEY"), LiveKitAPISecret: os.Getenv("LIVEKIT_API_SECRET")}
	service, err := New(cfg)
	if err != nil {
		t.Fatal("development voice configuration is invalid")
	}
	postID := uuid.New()
	admin := lksdk.NewRoomServiceClient(cfg.LiveKitURL, cfg.LiveKitAPIKey, cfg.LiveKitAPISecret)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.DeleteRoom(ctx, &lk.DeleteRoomRequest{Room: RoomName(postID)}); err != nil {
			t.Error("temporary room cleanup failed")
		}
	})
	var rooms []*lksdk.Room
	t.Cleanup(func() {
		for _, room := range rooms {
			room.Disconnect()
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var identities [4]uuid.UUID
	var joins [4]JoinCredentials
	received := make(chan int, 32)
	for i := range identities {
		identities[i] = uuid.New()
		joins[i], err = service.PrepareJoin(ctx, postID, identities[i], "接続テスト")
		if err != nil {
			t.Fatal("test participant preparation failed")
		}
		index := i
		room, err := lksdk.ConnectToRoomWithToken(cfg.LiveKitURL, joins[i].Token, &lksdk.RoomCallback{
			ParticipantCallback: lksdk.ParticipantCallback{
				OnTrackSubscribed: func(track *webrtc.TrackRemote, _ *lksdk.RemoteTrackPublication, _ *lksdk.RemoteParticipant) {
					go func() {
						for n := 0; n < 3; n++ {
							if _, _, err := track.ReadRTP(); err != nil {
								return
							}
						}
						select {
						case received <- index:
						default:
						}
					}()
				},
			},
		})
		if err != nil {
			t.Fatalf("test participant %d could not connect", i+1)
		}
		rooms = append(rooms, room)
	}
	people, err := service.Participants(ctx, postID)
	if err != nil || len(people) != 4 {
		t.Fatal("provider did not confirm four participants")
	}
	t.Log("four participants connected")
	listed, err := admin.ListRooms(ctx, &lk.ListRoomsRequest{Names: []string{RoomName(postID)}})
	if err != nil || len(listed.Rooms) != 1 {
		t.Fatal("test room could not be inspected")
	}
	t.Logf("provider room capacity=%d participants=%d", listed.Rooms[0].MaxParticipants, listed.Rooms[0].NumParticipants)

	fifth, err := service.PrepareJoin(ctx, postID, uuid.New(), "定員確認")
	if !errors.Is(err, ErrRoomFull) || fifth.Token != "" {
		t.Fatal("fifth participant was not rejected before token issuance")
	}
	t.Log("fifth participant denied a token using the live provider count")

	var tracks []*lksdk.LocalTrack
	for _, room := range rooms[:2] {
		track, err := lksdk.NewLocalSampleTrack(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2})
		if err != nil {
			t.Fatal("synthetic audio track creation failed")
		}
		t.Cleanup(func() { _ = track.Close() })
		if _, err := room.LocalParticipant.PublishTrack(track, &lksdk.TrackPublicationOptions{Name: "transport-test", Source: lk.TrackSource_MICROPHONE}); err != nil {
			t.Fatal("synthetic microphone publication failed")
		}
		tracks = append(tracks, track)
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	seen := map[int]bool{}
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	for !seen[0] || !seen[1] || !seen[2] || !seen[3] {
		select {
		case i := <-received:
			seen[i] = true
		case <-ticker.C:
			for _, track := range tracks {
				if err := track.WriteSample(media.Sample{Data: []byte{0xf8, 0xff, 0xfe}, Duration: 20 * time.Millisecond}, nil); err != nil {
					t.Fatal("synthetic audio transmission failed")
				}
			}
		case <-deadline.C:
			t.Fatal("audio packets did not reach all four participants")
		}
	}
	t.Log("bidirectional Opus transport and reception by all four participants confirmed")
	if err := service.Revoke(ctx, postID, identities[0]); err != nil {
		t.Fatal("connected participant revocation was not confirmed")
	}
	// The revocation cutoff intentionally includes the next second.
	time.Sleep(1200 * time.Millisecond)
	if room, err := lksdk.ConnectToRoomWithToken(cfg.LiveKitURL, joins[0].Token, nil); err == nil {
		room.Disconnect()
		t.Fatal("revoked token could reconnect")
	}
	t.Log("old join token rejected after removal")
	pendingID := uuid.New()
	if _, err := service.PrepareJoin(ctx, postID, pendingID, "未接続確認"); err != nil {
		t.Fatal("pending participant preparation failed")
	}
	replacement, err := service.PrepareJoin(ctx, postID, uuid.New(), "再入室確認")
	if err != nil {
		t.Fatal("replacement participant preparation failed")
	}
	room, err := lksdk.ConnectToRoomWithToken(cfg.LiveKitURL, replacement.Token, nil)
	if err != nil {
		t.Fatal("released slot could not be reused")
	}
	rooms = append(rooms, room)
	t.Log("released slot reused with a new identity")
	// Revoking an issued token before it ever joins must also be confirmed.
	if err := service.Revoke(ctx, postID, pendingID); err != nil {
		t.Fatal("pending participant revocation was not confirmed")
	}
	t.Log("pending token revocation confirmed")
}
