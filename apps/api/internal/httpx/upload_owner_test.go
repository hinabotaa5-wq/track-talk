package httpx_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

// 署名で発行された ID は、発行された本人だけが付けられる。他人の ID を付けると 422 になる。
func TestUploadIdsCanOnlyBeAttachedByTheUserTheyWereIssuedTo(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	owner, other := token(t, uuid.New()), token(t, uuid.New())

	rec := doJSON(t, h, http.MethodPost, "/v1/uploads/image", owner, map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("sign %d %s", rec.Code, rec.Body.String())
	}
	var signed struct {
		PublicID string `json:"public_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &signed)

	// 署名されていない ID は、誰が付けても通らない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/posts", owner, map[string]any{"title": "写真", "body": "", "image_public_id": "track-talk/" + uuid.NewString()}); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "cloudinary_public_id") {
		t.Fatalf("unsigned %d %s", rec.Code, rec.Body.String())
	}
	// 他人に発行された ID は付けられない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/posts", other, map[string]any{"title": "写真", "body": "", "image_public_id": signed.PublicID}); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "cloudinary_public_id") {
		t.Fatalf("other's id %d %s", rec.Code, rec.Body.String())
	}
}

// 添削用の非公開動画の ID も、発行された本人の依頼にしか付けられない。
func TestCoachingVideoIdsAreBoundToTheirOwner(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	owner := token(t, uuid.New())
	_, other := demoRequester(t, h, adminToken(t, h))
	id := signedCoachingVideo(t, h, owner)
	body := reviewBodyFor(t, h, other)
	body["video_public_id"] = id
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", other, body); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "video_public_id") {
		t.Fatalf("other's video %d %s", rec.Code, rec.Body.String())
	}
}

// 署名は1人1日 30 回まで。超えたら 429 になり、前の日の分は数えない。
func TestUploadSigningIsLimitedPerUserPerDay(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tok := token(t, uuid.New())
	first := signedCoachingVideo(t, h, tok)
	for i := 1; i < 30; i++ {
		grantUpload(t, h, tok, "track-talk/"+uuid.NewString())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/uploads/image", tok, map[string]any{}); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over limit %d %s", rec.Code, rec.Body.String())
	}
	execSQLArgs(t, `UPDATE cloudinary_uploads SET created_at = now() - interval '2 days' WHERE public_id = $1`, first)
	if rec := doJSON(t, h, http.MethodPost, "/v1/uploads/image", tok, map[string]any{}); rec.Code != http.StatusOK {
		t.Fatalf("after a day %d %s", rec.Code, rec.Body.String())
	}
}

// 同時に署名を頼んでも、1日 30 回を超えて出さない。回数を数えてから記録するまでのあいだに、別の署名を割り込ませない。
func TestUploadSigningLimitHoldsUnderConcurrency(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tok := token(t, uuid.New())
	for i := 0; i < 25; i++ {
		grantUpload(t, h, tok, "track-talk/"+uuid.NewString())
	}
	var wg sync.WaitGroup
	var ok atomic.Int32
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/v1/uploads/image", strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tok)
			// IP ごとの回数制限に当たらないよう、別々の IP から送る。
			req.RemoteAddr = fmt.Sprintf("203.0.113.%d:1234", i+1)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code == http.StatusOK {
				ok.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if got := ok.Load(); got != 5 {
		t.Fatalf("signed %d uploads concurrently, want 5", got)
	}
}
