package httpx_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 電気通信事業の届出が済むまで、有料の1対1のやりとりの送信は止まる。読むことは止めない。
func TestMessagingClosedBlocksPaidOneToOneSending(t *testing.T) {
	h := setupConfig(t, func(c *config.Config) { c.CoachingMessaging = false })
	admin := adminToken(t, h)
	_, owner := demoRequester(t, h, admin)

	id := uuid.NewString()
	for _, path := range []string{
		"/v1/coaching/contracts/" + id + "/messages",
		"/v1/guardian/secret/contracts/" + id + "/messages",
		"/v1/coaching/consultations/" + id + "/submit",
		"/v1/coaching/consultations/" + id + "/checkout",
		"/v1/guardian/secret/consultations/" + id + "/checkout",
		"/v1/coaching/consultations/" + id + "/answer",
		"/v1/coaching/consultations/" + id + "/clarify",
		"/v1/coaching/consultations/" + id + "/clarification",
		"/v1/coaching/requests/" + id + "/clarify",
		"/v1/coaching/requests/" + id + "/clarification",
	} {
		rec := doJSON(t, h, http.MethodPost, path, owner, map[string]any{"body": "フォームについて"})
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "messaging_closed") {
			t.Fatalf("closed %s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	// 取り消しはやりとりではないので止めない(この依頼は無いので 404 になる)。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/consultations/"+id+"/cancel", owner, nil); strings.Contains(rec.Body.String(), "messaging_closed") {
		t.Fatalf("cancel should not be blocked: %d %s", rec.Code, rec.Body.String())
	}
	// 読むほうは止めない(この契約は無いので 404 になる)。
	if rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id+"/messages", owner, nil); strings.Contains(rec.Body.String(), "messaging_closed") {
		t.Fatalf("reading should not be blocked: %d %s", rec.Code, rec.Body.String())
	}
}

// 日程の相談をやりとり欄で行う大会サポートは、やりとりを開くまで公開できない。下書きなら保存できる。
func TestMessagingClosedKeepsSchedulingPlansAsDrafts(t *testing.T) {
	h := setupConfig(t, func(c *config.Config) { c.CoachingMessaging = false })
	_, coach, plan := planCoach(t, h, 5)
	if r := doJSON(t, h, "PATCH", "/v1/coach/me/services", coach, map[string]any{"service_types": []string{"video_review", "competition_support"}}); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	body := planBody()
	body["service_type"] = "competition_support"
	body["competition_name"] = "秋季記録会"
	body["support_start_date"] = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	body["competition_date"] = time.Now().AddDate(0, 0, 14).Format("2006-01-02")
	body["video_reviews"] = 2
	body["call_count"] = 1
	body["call_minutes"] = 30
	body["price_yen"] = 9000
	body["published"] = true
	r := doJSON(t, h, "PATCH", "/v1/coach/me/plans/"+plan, coach, body)
	if r.Code != http.StatusUnprocessableEntity || !strings.Contains(r.Body.String(), "メッセージのやりとりの準備") {
		t.Fatalf("published support plan: %d %s", r.Code, r.Body.String())
	}
	body["published"] = false
	if r := doJSON(t, h, "PATCH", "/v1/coach/me/plans/"+plan, coach, body); r.Code != 200 {
		t.Fatalf("draft support plan: %d %s", r.Code, r.Body.String())
	}
}
