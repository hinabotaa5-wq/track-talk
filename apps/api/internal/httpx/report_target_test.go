package httpx_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// 投稿とコメントの通報は、読めるものだけ受け付ける。ない ID や消えたものを通報されても、運営の一覧が埋まるだけ。
func TestReportNeedsAReadableTarget(t *testing.T) {
	h := setup(t)
	tok := token(t, uuid.New())
	for _, kind := range []string{"post", "comment"} {
		rec := doJSON(t, h, http.MethodPost, "/v1/reports", tok, map[string]any{"target_type": kind, "target_id": uuid.NewString(), "reason": "spam"})
		if rec.Code != http.StatusNotFound {
			t.Errorf("report missing %s = %d %s", kind, rec.Code, rec.Body.String())
		}
	}
}
