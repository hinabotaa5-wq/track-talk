package httpx_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/httpx"
	"github.com/livekit/protocol/auth"
	lk "github.com/livekit/protocol/livekit"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const voiceKey = "voice-test-key"
const voiceSecret = "voice-test-secret"

type voiceHarness struct {
	mu          sync.Mutex
	h           http.Handler
	rooms       map[string]*lk.Room
	people      map[string][]*lk.ParticipantInfo
	revocations []string
	mutes       int
	fail        bool
}

func newVoiceHarness(t *testing.T) *voiceHarness {
	t.Helper()
	f := &voiceHarness{rooms: map[string]*lk.Room{}, people: map[string][]*lk.ParticipantInfo{}}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.fail {
			w.WriteHeader(503)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var response proto.Message
		switch {
		case strings.HasSuffix(r.URL.Path, "/CreateRoom"):
			req := &lk.CreateRoomRequest{}
			if err := proto.Unmarshal(body, req); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			if req.MaxParticipants != 4 {
				t.Error("room is not capped at four")
			}
			room := f.rooms[req.Name]
			if room == nil {
				room = &lk.Room{Sid: "RM-" + uuid.NewString(), Name: req.Name, MaxParticipants: req.MaxParticipants, CreationTime: time.Now().Unix()}
				f.rooms[req.Name] = room
			}
			response = room
		case strings.HasSuffix(r.URL.Path, "/ListParticipants"):
			req := &lk.ListParticipantsRequest{}
			if err := proto.Unmarshal(body, req); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			response = &lk.ListParticipantsResponse{Participants: f.people[req.Room]}
		case strings.HasSuffix(r.URL.Path, "/MutePublishedTrack"):
			req := &lk.MuteRoomTrackRequest{}
			if err := proto.Unmarshal(body, req); err != nil || !req.Muted {
				t.Error("remote unmute or malformed request")
				w.WriteHeader(400)
				return
			}
			f.mutes++
			response = &lk.MuteRoomTrackResponse{}
		case strings.HasSuffix(r.URL.Path, "/RemoveParticipant"):
			var req struct {
				Room     string `json:"room"`
				Identity string `json:"identity"`
				Cutoff   int64  `json:"revoke_token_ts"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			if req.Cutoff < time.Now().Unix() || req.Cutoff > time.Now().Add(2*time.Second).Unix() {
				t.Error("strict token revocation cutoff missing")
			}
			parsed, err := jwt.Parse(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), func(*jwt.Token) (any, error) { return []byte(voiceSecret), nil }, jwt.WithValidMethods([]string{"HS256"}))
			if err != nil {
				t.Error("invalid management signature")
				w.WriteHeader(401)
				return
			}
			if parsed.Claims.(jwt.MapClaims)["video"].(map[string]any)["room"] != req.Room {
				t.Error("management token has wrong room")
			}
			f.revocations = append(f.revocations, req.Identity)
			people := f.people[req.Room]
			kept := people[:0]
			for _, p := range people {
				if p.Identity != req.Identity {
					kept = append(kept, p)
				}
			}
			f.people[req.Room] = kept
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("{}"))
			return
		default:
			t.Error("unexpected provider API")
			w.WriteHeader(404)
			return
		}
		data, err := proto.Marshal(response)
		if err != nil {
			t.Error(err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/protobuf")
		_, _ = w.Write(data)
	}))
	t.Cleanup(provider.Close)
	f.h = setupConfig(t, func(cfg *config.Config) {
		cfg.MeetVoiceEnabled = true
		cfg.LiveKitURL = "wss://test.livekit.cloud"
		cfg.LiveKitAPIKey = voiceKey
		cfg.LiveKitAPISecret = voiceSecret
		cfg.LiveKitAPIBase = provider.URL
		cfg.MeetVoiceTickSecret = "voice-test-tick"
	})
	if _, err := openPool(t).Exec(context.Background(), `UPDATE meet_voice_settings SET paused=false,restrict_to_posts=false,last_tick_at=now(),monthly_minutes_limit=4000`); err != nil {
		t.Fatal(err)
	}
	return f
}
func voicePost(t *testing.T, kind string) uuid.UUID {
	t.Helper()
	pool := openPool(t)
	seedSystemProfile(t, pool)
	id := uuid.New()
	_, err := pool.Exec(context.Background(), `INSERT INTO posts(id,author_id,title,body,kind) VALUES ($1,$2,'通話検証','本文',$3)`, id, httpx.SystemProfileID, kind)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func voiceRequest(f *voiceHarness, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}
func joinVoice(f *voiceHarness, post uuid.UUID, token string) *httptest.ResponseRecorder {
	return voiceRequest(f, "POST", "/v1/posts/"+post.String()+"/voice/join", token, "")
}
func connectionID(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Code != 200 {
		t.Fatalf("join status %d", rec.Code)
	}
	var response struct {
		ID string `json:"connection_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil || response.ID == "" {
		t.Fatal("connection ID missing")
	}
	return response.ID
}

func TestMeetVoiceAuthorization(t *testing.T) {
	f := newVoiceHarness(t)
	meet := voicePost(t, "meet")
	authToken := tokenWithProfile(t, uuid.New(), "通話者", true)
	if rec := joinVoice(f, meet, ""); rec.Code != 401 {
		t.Fatalf("anonymous %d", rec.Code)
	}
	for _, post := range []uuid.UUID{voicePost(t, "text"), uuid.New()} {
		if rec := joinVoice(f, post, authToken); rec.Code != 404 {
			t.Fatalf("non-meet/missing %d", rec.Code)
		}
	}
	pool := openPool(t)
	for _, column := range []string{"hidden_at", "deleted_at"} {
		post := voicePost(t, "meet")
		if _, err := pool.Exec(context.Background(), "UPDATE posts SET "+column+"=now() WHERE id=$1", post); err != nil {
			t.Fatal(err)
		}
		if rec := joinVoice(f, post, authToken); rec.Code != 404 {
			t.Fatalf("hidden/deleted %d", rec.Code)
		}
	}
	user := uuid.New()
	bannedToken := token(t, user)
	if rec := voiceRequest(f, "GET", "/v1/me", bannedToken, ""); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO meet_voice_bans(post_id,user_id) VALUES ($1,$2)`, meet, user); err != nil {
		t.Fatal(err)
	}
	if rec := joinVoice(f, meet, bannedToken); rec.Code != 403 {
		t.Fatalf("banned %d", rec.Code)
	}
}

func TestMeetVoiceConcurrentCapacityAndLeave(t *testing.T) {
	f := newVoiceHarness(t)
	post := voicePost(t, "meet")
	tokens := make([]string, 5)
	for i := range tokens {
		tokens[i] = token(t, uuid.New())
		voiceRequest(f, "GET", "/v1/me", tokens[i], "")
	}
	results := make([]*httptest.ResponseRecorder, 5)
	var wg sync.WaitGroup
	for i := range tokens {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i] = joinVoice(f, post, tokens[i]) }(i)
	}
	wg.Wait()
	successes, full, winner, loser := 0, 0, -1, -1
	for i, rec := range results {
		switch rec.Code {
		case 200:
			successes++
			winner = i
		case 409:
			full++
			loser = i
		default:
			t.Fatalf("unexpected join %d", rec.Code)
		}
	}
	if successes != 4 || full != 1 {
		t.Fatalf("success %d full %d", successes, full)
	}
	id := connectionID(t, results[winner])
	path := "/v1/posts/" + post.String() + "/voice/leave"
	rec := voiceRequest(f, "POST", path, tokens[winner], `{"connection_id":"`+id+`"}`)
	if rec.Code != 204 {
		t.Fatalf("leave %d", rec.Code)
	}
	if rec := joinVoice(f, post, tokens[loser]); rec.Code != 200 {
		t.Fatalf("seat not released %d", rec.Code)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.revocations) != 1 || f.revocations[0] != id {
		t.Fatal("did not revoke the leaving identity")
	}
}

func TestMeetVoiceAccountSerializationAndPrivateName(t *testing.T) {
	f := newVoiceHarness(t)
	posts := []uuid.UUID{voicePost(t, "meet"), voicePost(t, "meet")}
	user := uuid.New()
	authToken := tokenWithProfile(t, user, "非公開名", false)
	voiceRequest(f, "GET", "/v1/me", authToken, "")
	results := make([]*httptest.ResponseRecorder, 2)
	var wg sync.WaitGroup
	for i := range posts {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i] = joinVoice(f, posts[i], authToken) }(i)
	}
	wg.Wait()
	success := 0
	for _, rec := range results {
		if rec.Code == 409 {
			continue
		}
		id := connectionID(t, rec)
		success++
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("token response may be cached")
		}
		var response struct {
			Token string `json:"token"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &response)
		parsed, err := jwt.Parse(response.Token, func(*jwt.Token) (any, error) { return []byte(voiceSecret), nil }, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil {
			t.Fatal("invalid join signature")
		}
		claims := parsed.Claims.(jwt.MapClaims)
		if claims["name"] != "匿名" || claims["sub"] != id || claims["sub"] == user.String() {
			t.Fatal("private profile leaked or identity not isolated")
		}
	}
	if success != 1 {
		t.Fatalf("successes %d", success)
	}
}

func TestMeetVoiceProviderCountsAndFailure(t *testing.T) {
	f := newVoiceHarness(t)
	post := voicePost(t, "meet")
	authToken := token(t, uuid.New())
	room := "meet-voice-" + post.String()
	f.people[room] = []*lk.ParticipantInfo{{Identity: uuid.NewString()}, {Identity: uuid.NewString()}, {Identity: uuid.NewString()}, {Identity: uuid.NewString()}}
	rec := voiceRequest(f, "GET", "/v1/posts/"+post.String()+"/voice", authToken, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"participants":4`) {
		t.Fatalf("provider count %d", rec.Code)
	}
	if rec := joinVoice(f, post, authToken); rec.Code != 409 {
		t.Fatalf("full provider room %d", rec.Code)
	}
	f.fail = true
	if rec := joinVoice(f, post, authToken); rec.Code != 503 {
		t.Fatalf("provider outage %d", rec.Code)
	}
}

func TestMeetVoiceExpiredReservationAndStaleLeave(t *testing.T) {
	f := newVoiceHarness(t)
	post := voicePost(t, "meet")
	user := uuid.New()
	authToken := token(t, user)
	old := connectionID(t, joinVoice(f, post, authToken))
	pool := openPool(t)
	if _, err := pool.Exec(context.Background(), `UPDATE meet_voice_reservations SET token_expires_at=now()-interval '1 minute' WHERE user_id=$1`, user); err != nil {
		t.Fatal(err)
	}
	newer := connectionID(t, joinVoice(f, post, authToken))
	if newer == old {
		t.Fatal("reused old identity")
	}
	path := "/v1/posts/" + post.String() + "/voice/leave"
	if rec := voiceRequest(f, "POST", path, authToken, `{"connection_id":"`+old+`"}`); rec.Code != 204 {
		t.Fatalf("stale leave %d", rec.Code)
	}
	if rec := joinVoice(f, post, authToken); rec.Code != 409 {
		t.Fatalf("stale leave removed new reservation %d", rec.Code)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.revocations) != 1 || f.revocations[0] != old {
		t.Fatal("old credential not revoked")
	}
}

func TestMeetVoiceRateLimit(t *testing.T) {
	f := newVoiceHarness(t)
	post := voicePost(t, "meet")
	authToken := token(t, uuid.New())
	connectionID(t, joinVoice(f, post, authToken))
	for range 4 {
		if rec := joinVoice(f, post, authToken); rec.Code != 409 {
			t.Fatal(rec.Code)
		}
	}
	if rec := joinVoice(f, post, authToken); rec.Code != 429 {
		t.Fatalf("rate limit %d", rec.Code)
	}
}

func signedVoiceEvent(t *testing.T, f *voiceHarness, event *lk.WebhookEvent, tamper bool) *httptest.ResponseRecorder {
	t.Helper()
	body, err := protojson.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	signed, err := auth.NewAccessToken(voiceKey, voiceSecret).SetValidFor(time.Minute).SetSha256(base64.StdEncoding.EncodeToString(digest[:])).ToJWT()
	if err != nil {
		t.Fatal("webhook signing failed")
	}
	if tamper {
		body = append(body, ' ')
	}
	req := httptest.NewRequest("POST", "/v1/livekit/webhook", bytes.NewReader(body))
	req.Header.Set("Authorization", signed)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func TestMeetVoiceWebhookSignatureDuplicateAndDelay(t *testing.T) {
	f := newVoiceHarness(t)
	post := voicePost(t, "meet")
	user := uuid.New()
	authToken := token(t, user)
	id := connectionID(t, joinVoice(f, post, authToken))
	now := time.Now().Unix()
	room := f.rooms["meet-voice-"+post.String()]
	event := &lk.WebhookEvent{Id: uuid.NewString(), Event: "participant_joined", CreatedAt: now - 2, Room: room, Participant: &lk.ParticipantInfo{Identity: id, Sid: "PA-new"}}
	if rec := voiceRequest(f, "POST", "/v1/livekit/webhook", "", `{}`); rec.Code != 401 {
		t.Fatal(rec.Code)
	}
	if rec := signedVoiceEvent(t, f, event, true); rec.Code != 401 {
		t.Fatalf("tampered %d", rec.Code)
	}
	for range 2 {
		if rec := signedVoiceEvent(t, f, event, false); rec.Code != 204 {
			t.Fatalf("valid/duplicate %d", rec.Code)
		}
	}
	pool := openPool(t)
	var count int
	var state string
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM meet_voice_webhook_events WHERE id=$1`, event.Id).Scan(&count); err != nil || count != 1 {
		t.Fatalf("dedup count %d err %v", count, err)
	}
	// Old participant SID and old timestamps must not replace the current state.
	for _, item := range []struct {
		sid string
		at  int64
	}{{"PA-old", now - 1}, {"PA-new", now - 3}} {
		event.Id = uuid.NewString()
		event.Event = "participant_left"
		event.Participant.Sid = item.sid
		event.CreatedAt = item.at
		if rec := signedVoiceEvent(t, f, event, false); rec.Code != 204 {
			t.Fatal(rec.Code)
		}
	}
	if err := pool.QueryRow(context.Background(), `SELECT state FROM meet_voice_reservations WHERE user_id=$1`, user).Scan(&state); err != nil || state != "connected" {
		t.Fatalf("stale event changed state %q %v", state, err)
	}
	event.Id = uuid.NewString()
	event.Participant.Sid = "PA-new"
	event.CreatedAt = now
	if rec := signedVoiceEvent(t, f, event, false); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	newer := connectionID(t, joinVoice(f, post, authToken))
	event.Id = uuid.NewString()
	event.CreatedAt = now + 1
	if rec := signedVoiceEvent(t, f, event, false); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	var stored string
	if err := pool.QueryRow(context.Background(), `SELECT id::text FROM meet_voice_reservations WHERE user_id=$1`, user).Scan(&stored); err != nil || stored != newer {
		t.Fatalf("old webhook removed new join %v", err)
	}
}

func TestMeetVoiceDisabled(t *testing.T) {
	h := setup(t)
	f := &voiceHarness{h: h}
	post := voicePost(t, "meet")
	authToken := token(t, uuid.New())
	rec := voiceRequest(f, "GET", "/v1/posts/"+post.String()+"/voice", authToken, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("disabled status %d", rec.Code)
	}
	if rec := joinVoice(f, post, authToken); rec.Code != 404 {
		t.Fatalf("disabled join %d", rec.Code)
	}
}

func TestMeetVoiceLeaveRequiresOwnershipAndConfirmedRevocation(t *testing.T) {
	f := newVoiceHarness(t)
	post := voicePost(t, "meet")
	user := uuid.New()
	owner := token(t, user)
	id := connectionID(t, joinVoice(f, post, owner))
	path := "/v1/posts/" + post.String() + "/voice/leave"
	other := token(t, uuid.New())
	if rec := voiceRequest(f, "POST", path, other, `{"connection_id":"`+id+`"}`); rec.Code != 204 {
		t.Fatalf("other leave %d", rec.Code)
	}
	if rec := voiceRequest(f, "POST", path, owner, `{}`); rec.Code != 400 {
		t.Fatalf("missing connection %d", rec.Code)
	}
	f.fail = true
	if rec := voiceRequest(f, "POST", path, owner, `{"connection_id":"`+id+`"}`); rec.Code != 503 {
		t.Fatalf("revocation failure %d", rec.Code)
	}
	pool := openPool(t)
	var stored string
	if err := pool.QueryRow(context.Background(), `SELECT id::text FROM meet_voice_reservations WHERE user_id=$1`, user).Scan(&stored); err != nil || stored != id {
		t.Fatalf("failed revoke released reservation %v", err)
	}
	f.fail = false
	// A hidden post still permits its own participant to leave.
	if _, err := pool.Exec(context.Background(), `UPDATE posts SET hidden_at=now() WHERE id=$1`, post); err != nil {
		t.Fatal(err)
	}
	if rec := voiceRequest(f, "POST", path, owner, `{"connection_id":"`+id+`"}`); rec.Code != 204 {
		t.Fatalf("hidden post leave %d", rec.Code)
	}
}

func TestMeetVoiceMissingWebhookUsesProviderPresence(t *testing.T) {
	f := newVoiceHarness(t)
	post := voicePost(t, "meet")
	user := uuid.New()
	authToken := token(t, user)
	id := connectionID(t, joinVoice(f, post, authToken))
	pool := openPool(t)
	if _, err := pool.Exec(context.Background(), `UPDATE meet_voice_reservations SET token_expires_at=now()-interval '1 minute' WHERE user_id=$1`, user); err != nil {
		t.Fatal(err)
	}
	f.people["meet-voice-"+post.String()] = []*lk.ParticipantInfo{{Identity: id, Sid: "PA-still-connected"}}
	if rec := joinVoice(f, post, authToken); rec.Code != 409 {
		t.Fatalf("missing webhook allowed duplicate %d", rec.Code)
	}
	if len(f.revocations) != 0 {
		t.Fatal("removed a still-connected participant")
	}
}

func TestMeetVoiceRoomFinishedOnlyClosesItsSession(t *testing.T) {
	f := newVoiceHarness(t)
	post := voicePost(t, "meet")
	authToken := token(t, uuid.New())
	connectionID(t, joinVoice(f, post, authToken))
	room := f.rooms["meet-voice-"+post.String()]
	pool := openPool(t)
	if _, err := pool.Exec(context.Background(), `INSERT INTO meet_voice_sessions(room_sid,post_id,room_name,started_at,last_event_at) VALUES ('RM-old',$1,$2,now()-interval '1 day',now()-interval '1 day')`, post, room.Name); err != nil {
		t.Fatal(err)
	}
	event := &lk.WebhookEvent{Id: uuid.NewString(), Event: "room_finished", CreatedAt: time.Now().Unix(), Room: &lk.Room{Sid: "RM-old", Name: room.Name, CreationTime: time.Now().Add(-24 * time.Hour).Unix()}}
	if rec := signedVoiceEvent(t, f, event, false); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	var oldClosed, newOpen bool
	if err := pool.QueryRow(context.Background(), `SELECT ended_at IS NOT NULL FROM meet_voice_sessions WHERE room_sid='RM-old'`).Scan(&oldClosed); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT ended_at IS NULL FROM meet_voice_sessions WHERE room_sid=$1`, room.Sid).Scan(&newOpen); err != nil {
		t.Fatal(err)
	}
	if !oldClosed || !newOpen {
		t.Fatal("old room event affected the new session")
	}
}

func TestMeetVoiceAbortedBeforeJoinWebhookRecovers(t *testing.T) {
	f := newVoiceHarness(t)
	post := voicePost(t, "meet")
	authToken := token(t, uuid.New())
	old := connectionID(t, joinVoice(f, post, authToken))
	room := f.rooms["meet-voice-"+post.String()]
	event := &lk.WebhookEvent{Id: uuid.NewString(), Event: "participant_connection_aborted", CreatedAt: time.Now().Unix(), Room: room, Participant: &lk.ParticipantInfo{Identity: old, Sid: "PA-aborted"}}
	if rec := signedVoiceEvent(t, f, event, false); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	newer := connectionID(t, joinVoice(f, post, authToken))
	if old == newer || len(f.revocations) != 1 || f.revocations[0] != old {
		t.Fatal("aborted pending join was not revoked and replaced")
	}
}

func TestMeetVoiceModerationAndReports(t *testing.T) {
	f := newVoiceHarness(t)
	post := voicePost(t, "meet")
	user := uuid.New()
	userToken := token(t, user)
	id := connectionID(t, joinVoice(f, post, userToken))
	second := token(t, uuid.New())
	connectionID(t, joinVoice(f, post, second))
	path := "/v1/admin/voice/connections/" + id
	if rec := voiceRequest(f, "POST", path, second, `{"action":"ban"}`); rec.Code != 403 {
		t.Fatalf("non-admin %d", rec.Code)
	}
	report := `{"target_type":"voice_participant","target_id":"` + id + `","reason":"harassment","detail":"通話中の迷惑行為"}`
	if rec := voiceRequest(f, "POST", "/v1/reports", token(t, uuid.New()), report); rec.Code != 403 {
		t.Fatalf("outsider report %d", rec.Code)
	}
	if rec := voiceRequest(f, "POST", "/v1/reports", second, report); rec.Code != 201 {
		t.Fatalf("voice report %d", rec.Code)
	}
	admin := adminToken(t, f.h)
	rec := voiceRequest(f, "GET", "/v1/admin/reports", admin, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "音声は保存していません") {
		t.Fatalf("admin report %d", rec.Code)
	}
	f.people["meet-voice-"+post.String()] = []*lk.ParticipantInfo{{Identity: id, Tracks: []*lk.TrackInfo{{Sid: "TR-mic", Source: lk.TrackSource_MICROPHONE}}}}
	if rec := voiceRequest(f, "POST", path, admin, `{"action":"mute"}`); rec.Code != 204 || f.mutes != 1 {
		t.Fatalf("mute %d", rec.Code)
	}
	if rec := voiceRequest(f, "POST", path, admin, `{"action":"unmute"}`); rec.Code != 400 {
		t.Fatalf("remote unmute %d", rec.Code)
	}
	if rec := voiceRequest(f, "POST", path, admin, `{"action":"ban","reason":"迷惑行為"}`); rec.Code != 204 {
		t.Fatalf("ban %d", rec.Code)
	}
	if rec := joinVoice(f, post, userToken); rec.Code != 403 {
		t.Fatalf("ban bypass %d", rec.Code)
	}
	if rec := voiceRequest(f, "POST", "/v1/admin/voice/bans/"+user.String()+"/remove", admin, `{"post_id":"`+post.String()+`"}`); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	connectionID(t, joinVoice(f, post, userToken))
}

func TestMeetVoiceGlobalStopRolloutAndQuota(t *testing.T) {
	f := newVoiceHarness(t)
	post := voicePost(t, "meet")
	authToken := token(t, uuid.New())
	connectionID(t, joinVoice(f, post, authToken))
	admin := adminToken(t, f.h)
	if rec := voiceRequest(f, "PUT", "/v1/admin/voice", admin, `{"paused":true,"restrict_to_posts":false,"allowed_post_ids":[],"monthly_minutes_limit":4000}`); rec.Code != 200 {
		t.Fatalf("pause %d", rec.Code)
	}
	if rec := joinVoice(f, post, token(t, uuid.New())); rec.Code != 503 {
		t.Fatalf("paused join %d", rec.Code)
	}
	if len(f.revocations) != 1 {
		t.Fatal("pause did not remove participant")
	}
	pool := openPool(t)
	if _, err := pool.Exec(context.Background(), `UPDATE meet_voice_settings SET paused=false,restrict_to_posts=true,allowed_post_ids='{}'`); err != nil {
		t.Fatal(err)
	}
	if rec := joinVoice(f, post, token(t, uuid.New())); rec.Code != 503 {
		t.Fatalf("rollout bypass %d", rec.Code)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE meet_voice_settings SET restrict_to_posts=false,monthly_minutes_limit=1`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE meet_voice_participants SET created_at=now()-interval '5 minutes',ended_at=now()`); err != nil {
		t.Fatal(err)
	}
	if rec := joinVoice(f, post, token(t, uuid.New())); rec.Code != 503 {
		t.Fatalf("quota bypass %d", rec.Code)
	}
}

func TestMeetVoiceHeartbeatConfirmationAndTick(t *testing.T) {
	f := newVoiceHarness(t)
	post := voicePost(t, "meet")
	user := uuid.New()
	authToken := token(t, user)
	id := connectionID(t, joinVoice(f, post, authToken))
	f.people["meet-voice-"+post.String()] = []*lk.ParticipantInfo{{Identity: id}}
	pool := openPool(t)
	if _, err := pool.Exec(context.Background(), `UPDATE meet_voice_reservations SET solo_since=now()-interval '15 minutes 30 seconds',confirmed_at=now()-interval '16 minutes'`); err != nil {
		t.Fatal(err)
	}
	path := "/v1/posts/" + post.String() + "/voice/heartbeat"
	rec := voiceRequest(f, "POST", path, authToken, `{"connection_id":"`+id+`"}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"confirmation_required":true`) {
		t.Fatalf("confirmation %d", rec.Code)
	}
	rec = voiceRequest(f, "POST", path, authToken, `{"connection_id":"`+id+`","confirm":true}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"confirmation_required":false`) {
		t.Fatalf("confirm action %d", rec.Code)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE meet_voice_reservations SET last_seen_at=now()-interval '2 minutes'`); err != nil {
		t.Fatal(err)
	}
	if rec := voiceRequest(f, "POST", "/internal/voice/tick", "", ""); rec.Code != 404 {
		t.Fatal("scheduler allowed without secret")
	}
	req := httptest.NewRequest("POST", "/internal/voice/tick", nil)
	req.Header.Set("X-Voice-Tick-Secret", "voice-test-tick")
	out := httptest.NewRecorder()
	f.h.ServeHTTP(out, req)
	if out.Code != 200 || len(f.revocations) != 1 {
		t.Fatalf("idle cleanup %d", out.Code)
	}
	if rec := voiceRequest(f, "POST", path, authToken, `{"connection_id":"`+id+`"}`); rec.Code != 410 {
		t.Fatalf("ended heartbeat %d", rec.Code)
	}
}

func TestMeetVoiceBanPersistsWhenProviderFails(t *testing.T) {
	f := newVoiceHarness(t)
	post := voicePost(t, "meet")
	userToken := token(t, uuid.New())
	id := connectionID(t, joinVoice(f, post, userToken))
	admin := adminToken(t, f.h)
	f.fail = true
	if rec := voiceRequest(f, "POST", "/v1/admin/voice/connections/"+id, admin, `{"action":"ban"}`); rec.Code != 503 {
		t.Fatal(rec.Code)
	}
	f.fail = false
	if rec := joinVoice(f, post, userToken); rec.Code != 403 {
		t.Fatal("failed removal lost participation ban")
	}
}

func TestMeetVoiceTickRevokesDeletedAccountIdentity(t *testing.T) {
	f := newVoiceHarness(t)
	post := voicePost(t, "meet")
	user := uuid.New()
	id := connectionID(t, joinVoice(f, post, token(t, user)))
	pool := openPool(t)
	if _, err := pool.Exec(context.Background(), `DELETE FROM profiles WHERE id=$1`, user); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/internal/voice/tick", nil)
	req.Header.Set("X-Voice-Tick-Secret", "voice-test-tick")
	out := httptest.NewRecorder()
	f.h.ServeHTTP(out, req)
	if out.Code != 200 || len(f.revocations) != 1 || f.revocations[0] != id {
		t.Fatalf("orphan cleanup %d", out.Code)
	}
}

func TestMeetVoiceStopsWhenSchedulerStalls(t *testing.T) {
	f := newVoiceHarness(t)
	post := voicePost(t, "meet")
	userToken := token(t, uuid.New())
	id := connectionID(t, joinVoice(f, post, userToken))
	f.people["meet-voice-"+post.String()] = []*lk.ParticipantInfo{{Identity: id}}
	if _, err := openPool(t).Exec(context.Background(), `UPDATE meet_voice_settings SET last_tick_at=now()-interval '3 minutes'`); err != nil {
		t.Fatal(err)
	}
	if rec := joinVoice(f, post, token(t, uuid.New())); rec.Code != 503 {
		t.Fatalf("stale scheduler allowed join %d", rec.Code)
	}
	if rec := voiceRequest(f, "POST", "/v1/posts/"+post.String()+"/voice/heartbeat", userToken, `{"connection_id":"`+id+`"}`); rec.Code != 410 {
		t.Fatalf("stale scheduler kept call %d", rec.Code)
	}
}
