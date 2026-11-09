package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// retentionCloud は、動画の削除を記録する Cloudinary の代わり。一覧は空、動画の詳細は配信できる状態で返す。
func retentionCloud(t *testing.T) (*config.Config, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var deleted []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			mu.Lock()
			deleted = append(deleted, r.URL.Query()["public_ids[]"]...)
			mu.Unlock()
			_, _ = w.Write([]byte(`{"deleted":{}}`))
			return
		}
		if strings.Contains(r.URL.Path, "/resources/video/authenticated/track-talk-coaching/") {
			_, _ = w.Write([]byte(`{"resource_type":"video","type":"authenticated","format":"mp4","bytes":1024,"duration":12}`))
			return
		}
		_, _ = w.Write([]byte(`{"resources":[]}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("CLOUDINARY_CLOUD_NAME", "demo")
	t.Setenv("CLOUDINARY_API_KEY", "key")
	t.Setenv("CLOUDINARY_API_SECRET", "secret")
	t.Setenv("CLOUDINARY_API_BASE", srv.URL)
	return nil, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(deleted)
	}
}

func coachingVideoID() string { return "track-talk-coaching/" + uuid.NewString() }

type seededReview struct {
	id, video, demo string
}

// seedReview は、動画添削の依頼を SQL で作る。returned は返却からの経過、clarified は確認の往復が終わってからの経過（"" は確認なし）。
func seedReview(t *testing.T, requester, coach uuid.UUID, status, returnedAgo, clarifiedAgo, updatedAgo string) seededReview {
	t.Helper()
	r := seededReview{id: uuid.NewString(), video: coachingVideoID(), demo: coachingVideoID()}
	execSQLArgs(t, `INSERT INTO review_requests (id, requester_id, tier, event, focus, video_public_id, status, due_at, coach_id, returned_at, updated_at)
		VALUES ($1, $2, 'coach', '100m', ARRAY['start'], $3, $4, now(), $5,
		        CASE WHEN $6 = '' THEN NULL ELSE now() - $6::interval END, now() - $7::interval)`,
		r.id, requester, r.video, status, coach, returnedAgo, updatedAgo)
	if status == "returned" {
		execSQLArgs(t, `INSERT INTO review_responses (request_id, coach_id, good_point, fix_points, drill, demo_video_public_id,
		        clarify_reason, clarify_requested_at, clarification, clarified_at)
			VALUES ($1, $2, 'よいところ', ARRAY['直すところ'], 'ドリル', $3,
			        CASE WHEN $4 = '' THEN NULL ELSE 'term' END, CASE WHEN $4 = '' THEN NULL ELSE now() - $4::interval - interval '1 hour' END,
			        CASE WHEN $4 = '' THEN NULL ELSE '補足です' END, CASE WHEN $4 = '' THEN NULL ELSE now() - $4::interval END)`,
			r.id, coach, r.demo, clarifiedAgo)
	}
	return r
}

type cleanupView struct {
	Videos struct {
		Reviews       int `json:"review_videos_purged"`
		Contracts     int `json:"contract_videos_purged"`
		Consultations int `json:"consultation_videos_purged"`
	} `json:"videos"`
}

// 動画添削はスレッドの終わりから7日、サポートプランは契約の終わりから30日で動画ファイルを消す。
// 通報・紛争の対応中（未対応の通報、運営の保全ホールド）と、進んでいる追加相談があるものは消さない。文章・メニューは残す。
func TestStorageCleanupPurgesVideosAfterRetention(t *testing.T) {
	_, deleted := retentionCloud(t)
	h := setupConfig(t, func(c *config.Config) { c.StorageCleanupSecret = "storage-secret" })
	tadmin := adminToken(t, h)

	requesterID, coachID := uuid.New(), uuid.New()
	tu := token(t, requesterID)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tu, nil); rec.Code != http.StatusOK {
		t.Fatalf("me %d", rec.Code)
	}
	execSQLArgs(t, `INSERT INTO profiles(id, nickname) VALUES ($1, 'coach-r')`, coachID)

	// 動画添削。確認を頼めるのは返却から7日なので、頼まれなかったときは返却から7日でスレッドが終わる。
	returned15 := seedReview(t, requesterID, coachID, "returned", "15 days", "", "15 days")      // 終わって8日 → 消す
	returned13 := seedReview(t, requesterID, coachID, "returned", "13 days", "", "13 days")      // 終わって6日 → 残す
	clarified8 := seedReview(t, requesterID, coachID, "returned", "20 days", "8 days", "8 days") // 確認の往復から8日 → 消す
	clarified6 := seedReview(t, requesterID, coachID, "returned", "20 days", "6 days", "6 days") // 確認の往復から6日 → 残す
	expired8 := seedReview(t, requesterID, coachID, "expired", "", "", "8 days")                 // 返却されずに終わって8日 → 消す
	expired6 := seedReview(t, requesterID, coachID, "expired", "", "", "6 days")                 // 6日 → 残す
	reported := seedReview(t, requesterID, coachID, "returned", "30 days", "", "30 days")        // 通報の対応中 → 残す
	execSQLArgs(t, `INSERT INTO reports (id, reporter_id, target_type, target_id, reason, detail) VALUES (gen_random_uuid(), $1, 'review_response', $2, 'other', '')`, requesterID, reported.id)
	held := seedReview(t, requesterID, coachID, "returned", "30 days", "", "30 days") // 運営の保全ホールド → 残す
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/retention-hold", tadmin, map[string]any{"target": "review", "id": held.id, "hold": true}); rec.Code != http.StatusOK {
		t.Fatalf("hold %d %s", rec.Code, rec.Body.String())
	}
	consulting := seedReview(t, requesterID, coachID, "returned", "30 days", "", "30 days") // 追加相談の回答待ち → 残す
	execSQLArgs(t, `INSERT INTO coaching_consultations (id, review_request_id, requester_id, coach_id, request_key, minor, funding, price_yen, share_percent, question, status, payment_state, paid_at, respond_by, answer_by, payment_intent_id)
		VALUES (gen_random_uuid(), $1, $2, $3, gen_random_uuid(), false, 'cash', 600, 70, '質問', 'in_review', 'paid', now(), now() + interval '1 day', now() + interval '1 day', 'pi_'||gen_random_uuid())`, consulting.id, requesterID, coachID)
	// 添削のあとの追加相談（回答して8日）。相談の質問と回答の動画も消す。
	answered := seedReview(t, requesterID, coachID, "returned", "13 days", "", "13 days")
	consultQ, consultA, consultID := coachingVideoID(), coachingVideoID(), uuid.NewString()
	execSQLArgs(t, `INSERT INTO coaching_consultations (id, review_request_id, requester_id, coach_id, request_key, minor, funding, price_yen, share_percent, question, status, payment_state, paid_at, answered_at, closed_at, answer, video_public_id, answer_video_public_id, payment_intent_id)
		VALUES ($1, $2, $3, $4, gen_random_uuid(), false, 'cash', 600, 70, '質問', 'answered', 'paid', now() - interval '9 days', now() - interval '8 days', now() - interval '8 days', '回答', $5, $6, 'pi_'||gen_random_uuid())`,
		consultID, answered.id, requesterID, coachID, consultQ, consultA)

	// サポートプラン。やりとり欄の動画を送ってから、終わりを動かす。
	contract := func() (string, string, string) {
		t.Helper()
		tc, tu, id := activeContract(t, h)
		video := signedCoachingVideo(t, h, tu)
		if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tu, map[string]any{"video_public_id": video}); rec.Code != http.StatusCreated {
			t.Fatalf("send video %d %s", rec.Code, rec.Body.String())
		}
		if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tc, map[string]any{"body": "1週目のメニューです"}); rec.Code != http.StatusCreated {
			t.Fatalf("send text %d", rec.Code)
		}
		return tu, id, video
	}
	_, ended31, ended31Video := contract()
	_, ended29, ended29Video := contract()
	_, refunded31, refunded31Video := contract()
	_, contractHeld, contractHeldVideo := contract()
	tuReported, contractReported, contractReportedVideo := contract()
	execSQLArgs(t, `UPDATE coaching_contracts SET status='ended', ends_at=now()-interval '31 days' WHERE id = ANY($1::uuid[])`, []string{ended31, contractHeld, contractReported})
	execSQLArgs(t, `UPDATE coaching_contracts SET status='ended', ends_at=now()-interval '29 days' WHERE id=$1`, ended29)
	execSQLArgs(t, `UPDATE coaching_contracts SET status='refunded', ends_at=now()-interval '31 days' WHERE id=$1`, refunded31)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/retention-hold", tadmin, map[string]any{"target": "contract", "id": contractHeld, "hold": true}); rec.Code != http.StatusOK {
		t.Fatalf("contract hold %d %s", rec.Code, rec.Body.String())
	}
	var coachMsg string
	queryRowArgs(t, `SELECT id::text FROM coaching_messages WHERE contract_id=$1 AND sender_role='coach'`, []any{contractReported}, &coachMsg)
	execSQLArgs(t, `INSERT INTO reports (id, reporter_id, target_type, target_id, reason, detail) SELECT gen_random_uuid(), requester_id, 'coaching_message', $2::uuid, 'other', '' FROM coaching_contracts WHERE id=$1`, contractReported, coachMsg)

	rec := cleanupRequest(h, "storage-secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("cleanup %d %s", rec.Code, rec.Body.String())
	}
	var out cleanupView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Videos.Reviews != 3 || out.Videos.Contracts != 2 || out.Videos.Consultations != 1 {
		t.Fatalf("counts %s", rec.Body.String())
	}
	got := deleted()
	for _, want := range []string{returned15.video, returned15.demo, clarified8.video, clarified8.demo, expired8.video, ended31Video, refunded31Video, consultQ, consultA} {
		if !slices.Contains(got, want) {
			t.Errorf("not deleted %s", want)
		}
	}
	for _, keep := range []string{returned13.video, returned13.demo, clarified6.video, expired6.video, reported.video, reported.demo, held.video, consulting.video, answered.video, ended29Video, contractHeldVideo, contractReportedVideo} {
		if slices.Contains(got, keep) {
			t.Errorf("deleted %s", keep)
		}
	}

	// 動画を消した依頼は、動画の URL を出さない。文章（返答）は残る。
	body := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+returned15.id, tu, nil).Body.String()
	if !strings.Contains(body, `"videos_purged":true`) || !strings.Contains(body, `"video_url":null`) || !strings.Contains(body, `"demo_video_url":null`) || !strings.Contains(body, "よいところ") {
		t.Fatalf("purged review %s", body)
	}
	if body := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+returned13.id, tu, nil).Body.String(); !strings.Contains(body, `"videos_purged":false`) || strings.Contains(body, `"video_url":null`) {
		t.Fatalf("kept review %s", body)
	}
	// 文章とメニューは消さない（保持は従来どおり1年）。
	var texts int
	queryRowArgs(t, `SELECT count(*)::int FROM coaching_messages WHERE contract_id=$1 AND body <> ''`, []any{ended31}, &texts)
	if texts != 1 {
		t.Fatalf("text kept %d", texts)
	}
	var purgedAt *string
	queryRowArgs(t, `SELECT videos_purged_at::text FROM coaching_contracts WHERE id=$1`, []any{ended31}, &purgedAt)
	if purgedAt == nil {
		t.Fatal("contract not marked")
	}
	// 保全で読めるまま残っている契約は、動画も残っている。
	if msgs := listMessages(t, h, tuReported, contractReported); len(msgs) != 2 {
		t.Fatalf("reported contract %d", len(msgs))
	}

	// 翌日は同じものを消し直さない。保全ホールドを外したものは、次の回で消す。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/retention-hold", tadmin, map[string]any{"target": "review", "id": held.id, "hold": false}); rec.Code != http.StatusOK {
		t.Fatalf("release %d", rec.Code)
	}
	rec = cleanupRequest(h, "storage-secret")
	out = cleanupView{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != http.StatusOK || out.Videos.Reviews != 1 || out.Videos.Contracts != 0 || out.Videos.Consultations != 0 {
		t.Fatalf("second cleanup %d %s", rec.Code, rec.Body.String())
	}
	if !slices.Contains(deleted(), held.video) {
		t.Fatal("released review not deleted")
	}
}
