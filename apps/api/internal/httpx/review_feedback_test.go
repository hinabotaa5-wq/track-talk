package httpx_test

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 返却後に任意で書ける匿名の感想（本体 #911）。

func feedbackCount(t *testing.T, requestID string) int {
	t.Helper()
	return scalar(t, `SELECT count(*) FROM review_feedbacks WHERE request_id = $1`, requestID)
}

func feedbackNoticeCount(t *testing.T, coachID uuid.UUID) int {
	t.Helper()
	return scalar(t, `SELECT count(*) FROM notifications WHERE recipient_id = $1 AND kind = 'message' AND title = '選手からの感想が届きました'`, coachID)
}

func feedbackSubmitted(t *testing.T, h http.Handler, tu, id string) bool {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+id, tu, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	v, ok := out["feedback_submitted"].(bool)
	if !ok {
		t.Fatalf("feedback_submitted missing %s", rec.Body.String())
	}
	return v
}

// coachFeedbacks は、コーチ側一覧の各要素をそのまま（キーごと）返す。
func coachFeedbacks(t *testing.T, h http.Handler, tc string) ([]map[string]any, string) {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coach/feedback", tc, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("coach feedback %d %s", rec.Code, rec.Body.String())
	}
	var out map[string][]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out["feedbacks"], rec.Body.String()
}

func TestReviewFeedback(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupConfig(t, func(cfg *config.Config) { cfg.CoachingTickSecret = "tick-secret" })
	tadmin := adminToken(t, h)
	coachID, tc := onDutyCoach(t, h, tadmin)
	otherCoachID, tother := onDutyCoach(t, h, tadmin)
	requesterID, tu := demoRequester(t, h, tadmin)
	id := acceptedReview(t, h, tadmin, tc, tu)
	url := "/v1/coaching/requests/" + id + "/feedback"
	const text = "フォームの癖が分かりやすく、ドリルもすぐ試せました"

	// A2: 返却前（添削中）は送れない。
	if rec := doJSON(t, h, http.MethodPost, url, tu, map[string]any{"body": text}); rec.Code != http.StatusConflict {
		t.Fatalf("in_review %d %s", rec.Code, rec.Body.String())
	}
	if n := feedbackCount(t, id); n != 0 {
		t.Fatalf("saved before return: %d", n)
	}

	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, responseBody()); rec.Code != http.StatusOK {
		t.Fatalf("response %d %s", rec.Code, rec.Body.String())
	}

	// A9: 送る前は false。
	if feedbackSubmitted(t, h, tu, id) {
		t.Fatal("feedback_submitted before submit")
	}

	// A3: 他人の依頼には送れない（404）。未認証では送れない。
	_, tstranger := demoRequester(t, h, tadmin)
	if rec := doJSON(t, h, http.MethodPost, url, tstranger, map[string]any{"body": text}); rec.Code != http.StatusNotFound {
		t.Fatalf("stranger %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, url, tc, map[string]any{"body": text}); rec.Code != http.StatusNotFound {
		t.Fatalf("coach as requester %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, url, "", map[string]any{"body": text}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated %d %s", rec.Code, rec.Body.String())
	}

	// A5: 空・空白だけ・501文字は 400（FieldError の形）。
	for _, bad := range []any{map[string]any{}, map[string]any{"body": ""}, map[string]any{"body": " \n\t　"}, map[string]any{"body": strings.Repeat("あ", 501)}} {
		rec := doJSON(t, h, http.MethodPost, url, tu, bad)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"validation_error"`) || !strings.Contains(rec.Body.String(), `"body"`) {
			t.Fatalf("bad body %v: %d %s", bad, rec.Code, rec.Body.String())
		}
	}
	// コーチをリクエストで指定させない（知らないフィールドは弾く）。
	if rec := doJSON(t, h, http.MethodPost, url, tu, map[string]any{"body": text, "coach_id": otherCoachID}); rec.Code != http.StatusBadRequest {
		t.Fatalf("coach_id in body %d %s", rec.Code, rec.Body.String())
	}
	if n := feedbackCount(t, id); n != 0 {
		t.Fatalf("saved invalid: %d", n)
	}

	// A1: 返却済みの自分の依頼に送れて、表示開始は送信の7日後。担当コーチはサーバ側で決まる。
	before := time.Now()
	rec := doJSON(t, h, http.MethodPost, url, tu, map[string]any{"body": "  " + text + "  "})
	if rec.Code != http.StatusCreated {
		t.Fatalf("submit %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Submitted bool      `json:"feedback_submitted"`
		VisibleAt time.Time `json:"visible_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || !created.Submitted {
		t.Fatalf("submit body %s", rec.Body.String())
	}
	if d := created.VisibleAt.Sub(before); d < 7*24*time.Hour-time.Minute || d > 7*24*time.Hour+time.Minute {
		t.Fatalf("visible_at %v (diff %v)", created.VisibleAt, d)
	}
	if n := scalar(t, `SELECT count(*) FROM review_feedbacks WHERE request_id = $1 AND coach_id = $2 AND body = $3 AND visible_at = created_at + interval '7 days' AND notified_at IS NULL`, id, coachID, text); n != 1 {
		t.Fatalf("stored row mismatch %d", n)
	}

	// A9: 送った後は true。
	if !feedbackSubmitted(t, h, tu, id) {
		t.Fatal("feedback_submitted after submit")
	}

	// A4: 2回目は 409 で、二重に保存されない。
	if rec := doJSON(t, h, http.MethodPost, url, tu, map[string]any{"body": "もう一度"}); rec.Code != http.StatusConflict {
		t.Fatalf("second %d %s", rec.Code, rec.Body.String())
	}
	if n := feedbackCount(t, id); n != 1 {
		t.Fatalf("double saved: %d", n)
	}

	// A6・A8: 表示開始前はコーチの一覧に出ず、tick を回しても通知しない。
	if items, _ := coachFeedbacks(t, h, tc); len(items) != 0 {
		t.Fatalf("visible before 7 days: %v", items)
	}
	if rec := tick(t, h, "tick-secret"); !strings.Contains(rec.Body.String(), `"feedback_notified":0`) {
		t.Fatalf("tick before visible %s", rec.Body.String())
	}
	if n := feedbackNoticeCount(t, coachID); n != 0 {
		t.Fatalf("notified before visible: %d", n)
	}
	// 6日23時間ではまだ見えない。
	execSQLArgs(t, `UPDATE review_feedbacks SET created_at = now() - interval '6 days 23 hours', visible_at = now() + interval '1 hour' WHERE request_id = $1`, id)
	if items, _ := coachFeedbacks(t, h, tc); len(items) != 0 {
		t.Fatalf("visible before visible_at: %v", items)
	}

	// 7日たつと見えるようになる。
	execSQLArgs(t, `UPDATE review_feedbacks SET created_at = now() - interval '7 days 1 minute', visible_at = now() - interval '1 minute' WHERE request_id = $1`, id)

	// A8: tick で1回だけ通知し、notified_at が立つ。もう一度回しても増えない。
	if rec := tick(t, h, "tick-secret"); !strings.Contains(rec.Body.String(), `"feedback_notified":1`) {
		t.Fatalf("tick visible %s", rec.Body.String())
	}
	if n := scalar(t, `SELECT count(*) FROM review_feedbacks WHERE request_id = $1 AND notified_at IS NOT NULL`, id); n != 1 {
		t.Fatalf("notified_at not set")
	}
	if n := feedbackNoticeCount(t, coachID); n != 1 {
		t.Fatalf("notice count %d", n)
	}
	if rec := tick(t, h, "tick-secret"); !strings.Contains(rec.Body.String(), `"feedback_notified":0`) {
		t.Fatalf("tick again %s", rec.Body.String())
	}
	if n := feedbackNoticeCount(t, coachID); n != 1 {
		t.Fatalf("notice count after second tick %d", n)
	}
	if n := feedbackNoticeCount(t, otherCoachID); n != 0 {
		t.Fatalf("other coach notified %d", n)
	}
	// 通知の文面・リンクに、感想の本文・依頼・選手の情報を含めない。
	var noticeBody, noticeURL string
	queryRowArgs(t, `SELECT body, url FROM notifications WHERE recipient_id = $1 AND title = '選手からの感想が届きました'`, []any{coachID}, &noticeBody, &noticeURL)
	if noticeBody != "新しい感想が届いています。" || noticeURL != "/coach/feedback" {
		t.Fatalf("notice %q %q", noticeBody, noticeURL)
	}
	if body := doJSON(t, h, http.MethodGet, "/v1/me/notifications", tc, nil).Body.String(); strings.Contains(body, text) || strings.Contains(body, requesterID.String()) {
		// 返却の通知など、ほかの知らせに依頼IDが入るのは既存の仕様なので、ここでは本文と選手IDだけを見る。
		t.Fatalf("notice leaks %s", body)
	}

	// A6・A7: 表示開始後は担当コーチの一覧に本文だけで出る。
	items, raw := coachFeedbacks(t, h, tc)
	if len(items) != 1 || items[0]["body"] != text {
		t.Fatalf("coach list %s", raw)
	}
	for _, it := range items {
		keys := make([]string, 0, len(it))
		for k := range it {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if strings.Join(keys, ",") != "body,id" {
			t.Fatalf("coach item keys %v (%s)", keys, raw)
		}
	}
	for _, leak := range []string{id, requesterID.String(), coachID.String(), "request_id", "requester", "coach_id", "created_at", "visible_at", "notified_at", "event", "video", "nickname", "date", time.Now().Format("2006-01-02")} {
		if strings.Contains(raw, leak) {
			t.Fatalf("coach list leaks %q: %s", leak, raw)
		}
	}
	// 他のコーチ宛ての感想は含まれない。
	if items, raw := coachFeedbacks(t, h, tother); len(items) != 0 {
		t.Fatalf("other coach sees %s", raw)
	}
	// コーチでない人は一覧を読めない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/coach/feedback", tu, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non coach list %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/coach/feedback", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list %d", rec.Code)
	}
	// 依頼の詳細（コーチ向け）にも感想の有無を出さない。
	if body := doJSON(t, h, http.MethodGet, "/v1/coach/requests/"+id, tc, nil).Body.String(); strings.Contains(body, "feedback") || strings.Contains(body, text) {
		t.Fatalf("coach request detail leaks feedback %s", body)
	}

	// A2・A5: clarifying でも送れる。500文字ちょうどは成功する。
	execSQLArgs(t, `DELETE FROM review_feedbacks WHERE request_id = $1`, id)
	execSQLArgs(t, `UPDATE review_requests SET status = 'clarifying' WHERE id = $1`, id)
	if rec := doJSON(t, h, http.MethodPost, url, tu, map[string]any{"body": strings.Repeat("あ", 500)}); rec.Code != http.StatusCreated {
		t.Fatalf("clarifying 500 %d %s", rec.Code, rec.Body.String())
	}
	if n := feedbackCount(t, id); n != 1 {
		t.Fatalf("clarifying saved %d", n)
	}
	// A2: returned／clarifying 以外の状態では 409 で保存されない。
	execSQLArgs(t, `DELETE FROM review_feedbacks WHERE request_id = $1`, id)
	for _, status := range []string{"queued", "offered", "in_review", "expired"} {
		execSQLArgs(t, `UPDATE review_requests SET status = $2 WHERE id = $1`, id, status)
		if rec := doJSON(t, h, http.MethodPost, url, tu, map[string]any{"body": text}); rec.Code != http.StatusConflict {
			t.Fatalf("status %s: %d %s", status, rec.Code, rec.Body.String())
		}
		if n := feedbackCount(t, id); n != 0 {
			t.Fatalf("status %s saved %d", status, n)
		}
	}
}
