package livekit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/joho/godotenv"
	lk "github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

// Explicit opt-in: creates and deletes one temporary room in the configured
// development project. Credentials and tokens must never appear in test output.
func TestCloudRoomLifecycle(t *testing.T) {
	if os.Getenv("LIVEKIT_TEST") != "1" {
		t.Skip("set LIVEKIT_TEST=1 to check the development LiveKit project")
	}
	_ = godotenv.Load("../../../../.env")
	cfg := config.Config{MeetVoiceEnabled: true, LiveKitURL: os.Getenv("LIVEKIT_URL"), LiveKitAPIKey: os.Getenv("LIVEKIT_API_KEY"), LiveKitAPISecret: os.Getenv("LIVEKIT_API_SECRET")}
	service, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	postID, userID := uuid.New(), uuid.New()
	client := lksdk.NewRoomServiceClient(cfg.LiveKitURL, cfg.LiveKitAPIKey, cfg.LiveKitAPISecret)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := client.DeleteRoom(ctx, &lk.DeleteRoomRequest{Room: "meet-voice-" + postID.String()}); err != nil {
			t.Error("temporary voice room cleanup failed")
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	join, err := service.PrepareJoin(ctx, postID, userID, "接続確認")
	if err != nil {
		t.Fatal("LiveKit room creation failed; check development credentials and service availability")
	}
	if join.Token == "" {
		t.Fatal("join token missing")
	}
	// A second request must keep the same room and capacity.
	if _, err := service.PrepareJoin(ctx, postID, uuid.New(), "接続確認2"); err != nil {
		t.Fatal("existing room verification failed")
	}
}
