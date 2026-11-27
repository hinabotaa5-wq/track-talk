package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type coachRequestsView struct {
	Offers []struct {
		ID      string            `json:"id"`
		Request reviewRequestView `json:"request"`
	} `json:"offers"`
	Reviewing []reviewRequestView `json:"reviewing"`
}

// approvedReviewCoach は、動画添削に必要な承認と振込先を設定する。
func approvedReviewCoach(t *testing.T, h http.Handler, tadmin string) (uuid.UUID, string) {
	t.Helper()
	return approvedReviewCoachID(t, h, tadmin, uuid.New())
}

// approvedReviewCoachID は approvedReviewCoach の ID を先に決める版（特集モードのテストで使う）。
func approvedReviewCoachID(t *testing.T, h http.Handler, tadmin string, id uuid.UUID) (uuid.UUID, string) {
	t.Helper()
	tc := token(t, id)
	body := coachApplicationBody()
	body["tier"] = "coach"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tc, body); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	withCoachPhoto(t, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+id.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d", rec.Code)
	}
	// 支払いのあるテストでも受付を始められるように、受け取り口座の登録を済ませておく。
	execSQLArgs(t, `UPDATE coach_profiles SET stripe_account_id = $2, payouts_enabled = true WHERE profile_id = $1`, id, "acct_"+id.String())
	return id, tc
}

// onDutyCoach は、承認済みで受付中のコーチ（枠はコーチ、種目は110mハードルを含む）を作る。
func onDutyCoach(t *testing.T, h http.Handler, tadmin string) (uuid.UUID, string) {
	t.Helper()
	return onDutyCoachID(t, h, tadmin, uuid.New())
}

// onDutyCoachID は onDutyCoach の ID を先に決める版（特集モードのテストで使う）。
func onDutyCoachID(t *testing.T, h http.Handler, tadmin string, want uuid.UUID) (uuid.UUID, string) {
	t.Helper()
	id, tc := approvedReviewCoachID(t, h, tadmin, want)
	execSQLArgs(t, `INSERT INTO push_subscriptions (id, profile_id, endpoint, p256dh, auth) VALUES ($1, $2, $3, 'key', 'auth')`,
		uuid.New(), id, "https://push.example.com/"+uuid.NewString())
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("on duty %d %s", rec.Code, rec.Body.String())
	}
	return id, tc
}

func coachRequests(t *testing.T, h http.Handler, tc string) coachRequestsView {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coach/requests", tc, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("coach requests %d %s", rec.Code, rec.Body.String())
	}
	var out coachRequestsView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func reviewStatus(t *testing.T, h http.Handler, tu, id string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+id, tu, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}
	var out reviewRequestView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.Status
}

func tick(t *testing.T, h http.Handler, secret string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/internal/coaching/tick", nil)
	if secret != "" {
		req.Header.Set("X-Coaching-Tick-Secret", secret)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func dueIn24h(due, before, after time.Time) bool {
	return !due.Before(before.Add(24*time.Hour).Truncate(time.Second)) && !due.After(after.Add(24*time.Hour))
}

func TestReviewStartLocksCancellation(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coachID, coach := onDutyCoach(t, h, admin)
	_, athlete := demoRequester(t, h, admin)

	id := chosenReview(t, h, athlete, coachID.String(), "card", false)
	payChosenReview(t, h, id, "card")
	if got := coachRequests(t, h, coach).Reviewing; len(got) != 1 || got[0].ID != id || got[0].VideoURL == "" {
		t.Fatalf("request must immediately appear with its video: %+v", got)
	}
	var assigned, due time.Time
	queryRowArgs(t, "SELECT assigned_at, due_at FROM review_requests WHERE id=$1", []any{id}, &assigned, &due)
	if due.Sub(assigned) != 24*time.Hour {
		t.Fatalf("due %s, assigned %s", due, assigned)
	}
	athletePath := "/v1/coaching/requests/" + id
	coachPath := "/v1/coach/requests/" + id
	if rec := doJSON(t, h, "POST", athletePath+"/response", coach, responseBody()); rec.Code != http.StatusConflict {
		t.Fatalf("response before start: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, h, "POST", coachPath+"/start", coach, nil); rec.Code != http.StatusOK {
		t.Fatalf("start: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, h, "POST", athletePath+"/cancel", athlete, nil); rec.Code != http.StatusConflict {
		t.Fatalf("cancel after start: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, h, "POST", athletePath+"/response", coach, responseBody()); rec.Code != http.StatusOK {
		t.Fatalf("response after start: %d %s", rec.Code, rec.Body)
	}
}

// 履歴（本体 #1014）の要素。一覧では動画を出さないので、動画のキーは生の JSON で有無を見る。
type completedItemView struct {
	ID         string  `json:"id"`
	Status     string  `json:"status"`
	ReturnedAt *string `json:"returned_at"`
}

// coachRequestsRaw は、GET /v1/coach/requests をキーごとに読む。
func coachRequestsRaw(t *testing.T, h http.Handler, tc string) map[string]json.RawMessage {
	t.Helper()
	return coachRequestsRawPath(t, h, tc, "/v1/coach/requests")
}

// coachRequestsRawPath は coachRequestsRaw のクエリ付き版（本体 #1071 の scope）。
func coachRequestsRawPath(t *testing.T, h http.Handler, tc, path string) map[string]json.RawMessage {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, path, tc, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("coach requests %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func coachCompleted(t *testing.T, h http.Handler, tc string) []completedItemView {
	t.Helper()
	var out []completedItemView
	if err := json.Unmarshal(coachRequestsRaw(t, h, tc)["completed"], &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// seedHistoryReview は、履歴のテスト用に依頼を SQL で作る（依頼した人は別人のランダムな ID。actionSeed で外部キーを外す）。
// returnedAgo が "" なら returned_at は入れない。動画は付けておく（一覧に出ないことを確かめるため）。
func seedHistoryReview(t *testing.T, coach uuid.UUID, status, returnedAgo string) string {
	t.Helper()
	id := uuid.NewString()
	actionSeed(t, `INSERT INTO review_requests (id, requester_id, tier, event, focus, question, video_public_id, status, due_at, coach_id,
			returned_at, info_request, info_requested_at, info_from_status, resubmit_by)
		VALUES ($1, gen_random_uuid(), 'coach', '100m', ARRAY['start'], 'スタートを見てください', $2, $3, now() + interval '1 day', $4,
			CASE WHEN $5 = '' THEN NULL ELSE now() - $5::interval END,
			CASE WHEN $3 = 'needs_info' THEN '質問' END, CASE WHEN $3 = 'needs_info' THEN now() END,
			CASE WHEN $3 = 'needs_info' THEN 'in_review' END, CASE WHEN $3 = 'needs_info' THEN now() + interval '1 day' END)`,
		id, coachingVideoID(), status, coach, returnedAgo)
	return id
}

// #1014 A1〜A5・A8：履歴（completed）の中身と並び。一覧では動画を出さない（運営者決定 2026-11-25）。
func TestCoachRequestsCompleted(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	admin := adminToken(t, h)
	coachID, coach := approvedReviewCoach(t, h, admin)
	otherID, _ := approvedReviewCoach(t, h, admin)

	returnedOld := seedHistoryReview(t, coachID, "returned", "3 hours")
	returnedNew := seedHistoryReview(t, coachID, "returned", "1 hour")
	clarifying := seedHistoryReview(t, coachID, "clarifying", "2 hours")
	seedHistoryReview(t, coachID, "in_review", "")
	seedHistoryReview(t, coachID, "needs_info", "")
	seedHistoryReview(t, coachID, "expired", "")
	withdrawn := seedHistoryReview(t, coachID, "expired", "")
	execSQLArgs(t, `UPDATE review_requests SET withdrawn_at = now(), withdrawn_from_status = 'in_review' WHERE id = $1`, withdrawn) // 辞退済み
	seedHistoryReview(t, otherID, "returned", "30 minutes")                                                                         // A4：別のコーチ

	raw := coachRequestsRaw(t, h, coach)
	// A8：offers・reviewing は従来どおり。reviewing は動画付き、returned_at は null。
	if string(raw["offers"]) != "[]" {
		t.Fatalf("A8 offers = %s", raw["offers"])
	}
	var reviewing []map[string]json.RawMessage
	if err := json.Unmarshal(raw["reviewing"], &reviewing); err != nil {
		t.Fatal(err)
	}
	if len(reviewing) != 2 {
		t.Fatalf("A8 reviewing = %d件, want 2（in_review・needs_info）", len(reviewing))
	}
	for _, item := range reviewing {
		var id string
		_ = json.Unmarshal(item["id"], &id)
		if id == returnedOld || id == returnedNew || id == clarifying {
			t.Fatalf("A1 returned/clarifying in reviewing: %s", id)
		}
		if v, ok := item["returned_at"]; !ok || string(v) != "null" {
			t.Fatalf("reviewing returned_at = %s (present %v)", v, ok)
		}
		if v := string(item["video_url"]); v == "" || v == "null" {
			t.Fatalf("A8 reviewing video_url = %q", v)
		}
	}

	// A1・A2・A3・A4・A5：returned と clarifying だけが、returned_at の新しい順。
	got := coachCompleted(t, h, coach)
	want := []string{returnedNew, clarifying, returnedOld}
	if len(got) != len(want) {
		t.Fatalf("completed = %+v, want %v", got, want)
	}
	for i, item := range got {
		if item.ID != want[i] {
			t.Fatalf("A5 order[%d] = %s, want %s (all %+v)", i, item.ID, want[i], got)
		}
		if item.ReturnedAt == nil || *item.ReturnedAt == "" {
			t.Fatalf("A1 returned_at missing: %+v", item)
		}
	}
	if got[1].Status != "clarifying" {
		t.Fatalf("A2 status = %s", got[1].Status)
	}
	// 一覧では動画を出さない：video_url・thumbnail_url・demo_video_url のキーが付かない。
	var completedRaw []map[string]json.RawMessage
	if err := json.Unmarshal(raw["completed"], &completedRaw); err != nil {
		t.Fatal(err)
	}
	for _, item := range completedRaw {
		for _, key := range []string{"video_url", "thumbnail_url", "demo_video_url"} {
			if _, ok := item[key]; ok {
				t.Fatalf("completed has %s: %v", key, item)
			}
		}
	}
}

// #1071：scope=reviewing は対応中だけ、scope=completed は履歴だけ。scope なし・未知の値は従来どおり両方。
func TestCoachRequestsScope(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	admin := adminToken(t, h)
	coachID, coach := approvedReviewCoach(t, h, admin)
	returned := seedHistoryReview(t, coachID, "returned", "1 hour")
	clarifying := seedHistoryReview(t, coachID, "clarifying", "2 hours")
	inReview := seedHistoryReview(t, coachID, "in_review", "")
	needsInfo := seedHistoryReview(t, coachID, "needs_info", "")

	ids := func(t *testing.T, raw json.RawMessage) []string {
		t.Helper()
		var items []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(items))
		for _, item := range items {
			var id string
			_ = json.Unmarshal(item["id"], &id)
			out = append(out, id)
		}
		return out
	}
	sameSet := func(got []string, want ...string) bool {
		if len(got) != len(want) {
			return false
		}
		seen := map[string]bool{}
		for _, id := range got {
			seen[id] = true
		}
		for _, id := range want {
			if !seen[id] {
				return false
			}
		}
		return true
	}
	keys := func(raw map[string]json.RawMessage) bool {
		_, o := raw["offers"]
		_, r := raw["reviewing"]
		_, c := raw["completed"]
		return len(raw) == 3 && o && r && c && string(raw["offers"]) == "[]"
	}

	// 基準：scope なし（従来どおり両方）。
	base := coachRequestsRaw(t, h, coach)
	if !keys(base) {
		t.Fatalf("A3 keys = %v", base)
	}
	if got := ids(t, base["reviewing"]); !sameSet(got, inReview, needsInfo) {
		t.Fatalf("A3 reviewing = %v", got)
	}
	if got := ids(t, base["completed"]); !sameSet(got, returned, clarifying) {
		t.Fatalf("A3 completed = %v", got)
	}

	// A1：scope=reviewing。completed は空配列、reviewing は scope なしと同じ中身（動画・サムネイルURL付き）。
	rev := coachRequestsRawPath(t, h, coach, "/v1/coach/requests?scope=reviewing")
	if !keys(rev) || string(rev["completed"]) != "[]" {
		t.Fatalf("A1 = %v", rev)
	}
	if string(rev["reviewing"]) != string(base["reviewing"]) {
		t.Fatalf("A1 reviewing differs:\n%s\n%s", rev["reviewing"], base["reviewing"])
	}
	var revItems []map[string]json.RawMessage
	if err := json.Unmarshal(rev["reviewing"], &revItems); err != nil {
		t.Fatal(err)
	}
	for _, item := range revItems {
		for _, key := range []string{"video_url", "thumbnail_url"} {
			if v := string(item[key]); v == "" || v == "null" {
				t.Fatalf("A1 reviewing %s = %q", key, v)
			}
		}
	}

	// A2：scope=completed。reviewing は空配列、completed は scope なしと同じ中身（動画なし）。
	comp := coachRequestsRawPath(t, h, coach, "/v1/coach/requests?scope=completed")
	if !keys(comp) || string(comp["reviewing"]) != "[]" {
		t.Fatalf("A2 = %v", comp)
	}
	if string(comp["completed"]) != string(base["completed"]) {
		t.Fatalf("A2 completed differs:\n%s\n%s", comp["completed"], base["completed"])
	}
	var compItems []map[string]json.RawMessage
	if err := json.Unmarshal(comp["completed"], &compItems); err != nil {
		t.Fatal(err)
	}
	for _, item := range compItems {
		for _, key := range []string{"video_url", "thumbnail_url", "demo_video_url"} {
			if _, ok := item[key]; ok {
				t.Fatalf("A2 completed has %s", key)
			}
		}
	}

	// A3：未知の値・空の値は従来どおり両方。
	for _, path := range []string{"/v1/coach/requests?scope=unknown", "/v1/coach/requests?scope=", "/v1/coach/requests?scope=REVIEWING"} {
		raw := coachRequestsRawPath(t, h, coach, path)
		if !keys(raw) || string(raw["reviewing"]) != string(base["reviewing"]) || string(raw["completed"]) != string(base["completed"]) {
			t.Fatalf("A3 %s = %v", path, raw)
		}
	}
}

// #1014 A6：上限100件。
func TestCoachRequestsCompletedLimit(t *testing.T) {
	h := setup(t)
	admin := adminToken(t, h)
	coachID, coach := approvedReviewCoach(t, h, admin)
	actionSeed(t, `INSERT INTO review_requests (id, requester_id, tier, event, focus, question, status, due_at, coach_id, returned_at)
		SELECT gen_random_uuid(), gen_random_uuid(), 'coach', '100m', ARRAY['start'], 'スタートを見てください', 'returned', now(), $1,
			now() - make_interval(mins => g)
		FROM generate_series(1, 101) AS g`, coachID)
	got := coachCompleted(t, h, coach)
	if len(got) != 100 {
		t.Fatalf("A6 completed = %d件, want 100", len(got))
	}
	// 一番古い（101分前）が落ちる。
	var oldest string
	queryRowArgs(t, `SELECT id::text FROM review_requests WHERE coach_id = $1 ORDER BY returned_at LIMIT 1`, []any{coachID}, &oldest)
	for _, item := range got {
		if item.ID == oldest {
			t.Fatalf("A6 oldest must be cut")
		}
	}
}

// #1014 A7：未承認コーチ・コーチでない人には completed を返さない。
func TestCoachRequestsCompletedRequiresApprovedCoach(t *testing.T) {
	h := setup(t)
	// コーチでない人。
	stranger := token(t, uuid.New())
	if rec := doJSON(t, h, http.MethodGet, "/v1/coach/requests", stranger, nil); rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), "completed") {
		t.Fatalf("A7 non-coach: %d %s", rec.Code, rec.Body.String())
	}
	// 申し込んだだけで未承認のコーチ。返却済みの依頼が担当に付いていても見せない。
	pendingID := uuid.New()
	pending := token(t, pendingID)
	body := coachApplicationBody()
	body["tier"] = "coach"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", pending, body); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	seedHistoryReview(t, pendingID, "returned", "1 hour")
	if rec := doJSON(t, h, http.MethodGet, "/v1/coach/requests", pending, nil); rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), "completed") {
		t.Fatalf("A7 pending coach: %d %s", rec.Code, rec.Body.String())
	}
}
