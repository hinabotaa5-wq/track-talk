package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// acceptedReview は、コーチが引き受けて添削中になった依頼を1つ作る。
func acceptedReview(t *testing.T, h http.Handler, tadmin, tc, tu string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBodyFor(t, h, tu))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created reviewRequestView
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	offers := coachRequests(t, h, tc).Reviewing
	if len(offers) == 0 {
		t.Fatal("no offer")
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+offers[0].ID+"/start", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	return created.ID
}

func demoRequester(t *testing.T, h http.Handler, tadmin string) (uuid.UUID, string) {
	t.Helper()
	id := uuid.New()
	tu := token(t, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/demo", tu, map[string]any{"code": newDemoCodeFor(t, h, tadmin)}); rec.Code != http.StatusOK {
		t.Fatalf("join %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/birth-month", tu, map[string]any{"birth_month": "1995-04"}); rec.Code != http.StatusOK {
		t.Fatalf("birth month %d", rec.Code)
	}
	return id, tu
}

func responseBody() map[string]any {
	return map[string]any{
		"good_point": "1台目までの8歩が安定しています",
		"fix_points": []string{"踏切が近いので、半足ぶん手前で踏み切りましょう", " ", "抜き脚の膝を高く"},
		"drill":      "ミニハードル5台を1.5m間隔で並べて、3セット",
		"hitokoto":   "次は抜き脚だけ意識してみましょう",
	}
}

func TestReviewResponseFlow(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupConfig(t, func(cfg *config.Config) { cfg.CoachingTickSecret = "tick-secret" })
	tadmin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, tadmin)
	_, tother := onDutyCoach(t, h, tadmin)
	_, tu := demoRequester(t, h, tadmin)
	id := acceptedReview(t, h, tadmin, tc, tu)

	// 連絡先らしき文字列があると止める。
	bad := responseBody()
	bad["good_point"] = "続きは coach@example.com に送ります"
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, bad)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"good_point"`) {
		t.Fatalf("contact %d %s", rec.Code, rec.Body.String())
	}
	bad = responseBody()
	bad["fix_points"] = []string{"a", "b", "c"}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, bad); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("3 fixes %d", rec.Code)
	}
	// ひとことが無い・空・長すぎ・連絡先らしき内容だと止める。
	bad = responseBody()
	delete(bad, "hitokoto")
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, bad); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"hitokoto"`) {
		t.Fatalf("missing hitokoto %d %s", rec.Code, rec.Body.String())
	}
	bad = responseBody()
	bad["hitokoto"] = "   "
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, bad); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"hitokoto"`) {
		t.Fatalf("empty hitokoto %d %s", rec.Code, rec.Body.String())
	}
	bad = responseBody()
	bad["hitokoto"] = strings.Repeat("あ", 301)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, bad); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"hitokoto"`) {
		t.Fatalf("long hitokoto %d %s", rec.Code, rec.Body.String())
	}
	bad = responseBody()
	bad["hitokoto"] = "続きは coach@example.com に送ります"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, bad); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"hitokoto"`) {
		t.Fatalf("hitokoto contact %d %s", rec.Code, rec.Body.String())
	}
	// 引き受けていないコーチは返答できない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tother, responseBody()); rec.Code != http.StatusConflict {
		t.Fatalf("other coach %d", rec.Code)
	}
	// 返答の前は★をつけられない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/rating", tu, map[string]any{"stars": 4}); rec.Code != http.StatusConflict {
		t.Fatalf("rate before response %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, responseBody())
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"returned"`) {
		t.Fatalf("response %d %s", rec.Code, rec.Body.String())
	}
	// 直す点2つ（空の3つ目は捨てる）でも保存できる。
	var saved map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatalf("response json %v", err)
	}
	resp, _ := saved["response"].(map[string]any)
	fixes, _ := resp["fix_points"].([]any)
	if len(fixes) != 2 {
		t.Fatalf("fix_points len %d, want 2 (%s)", len(fixes), rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, responseBody()); rec.Code != http.StatusConflict {
		t.Fatalf("second response %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+id, tu, nil)
	body := rec.Body.String()
	if !strings.Contains(body, `"status":"returned"`) || !strings.Contains(body, "抜き脚の膝を高く") || strings.Contains(body, `" "`) {
		t.Fatalf("requester view %s", body)
	}
	if !strings.Contains(body, `"hitokoto":"次は抜き脚だけ意識してみましょう"`) {
		t.Fatalf("requester hitokoto %s", body)
	}
	// 返答と一緒にコーチのプロフィールが届く。
	if !strings.Contains(body, `"best_record":"100m 10秒95（2025年）"`) || !strings.Contains(body, `"achievements":"インターハイ 100m 準決勝"`) {
		t.Fatalf("coach profile %s", body)
	}
	if !strings.Contains(doJSON(t, h, http.MethodGet, "/v1/me/notifications", tu, nil).Body.String(), `"kind":"review_returned"`) {
		t.Fatal("no returned notification")
	}

	// ★は1〜5でつけ直せる。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/rating", tu, map[string]any{"stars": 6}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("stars 6 %d", rec.Code)
	}
	doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/rating", tu, map[string]any{"stars": 3})
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/rating", tu, map[string]any{"stars": 4})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"stars":4`) {
		t.Fatalf("stars %d %s", rec.Code, rec.Body.String())
	}
	if body := doJSON(t, h, http.MethodGet, "/v1/coach/me", tc, nil).Body.String(); !strings.Contains(body, `"rating":{"average":4,"count":1,"paused":false}`) {
		t.Fatalf("coach rating %s", body)
	}

	// メッセージを開放しても、無料の確認質問と補足の送信先は存在しない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/clarify", tu, map[string]any{"reason": "drill"}); rec.Code != http.StatusNotFound {
		t.Fatalf("retired clarification request %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/clarification", tc, map[string]any{"clarification": "補足"}); rec.Code != http.StatusNotFound {
		t.Fatalf("retired clarification response %d %s", rec.Code, rec.Body.String())
	}

	// 廃止前に保存されたやりとりは双方の詳細画面で読める。
	execSQLArgs(t, `UPDATE review_responses SET clarify_reason='drill', clarify_body='ドリルの確認', clarify_requested_at=now(), clarification='抜き脚だけを意識', clarified_at=now() WHERE request_id=$1`, id)
	for _, viewer := range []struct{ path, token string }{
		{"/v1/coaching/requests/" + id, tu},
		{"/v1/coach/requests/" + id, tc},
	} {
		rec := doJSON(t, h, http.MethodGet, viewer.path, viewer.token, nil)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ドリルの確認") || !strings.Contains(rec.Body.String(), "抜き脚だけを意識") {
			t.Fatalf("historical clarification %d %s", rec.Code, rec.Body.String())
		}
	}

	// 7日たつと「役に立ちましたか」を一度だけ聞く。
	execSQLArgs(t, `UPDATE review_requests SET returned_at = now() - interval '8 days' WHERE id = $1`, id)
	if rec := tick(t, h, "tick-secret"); !strings.Contains(rec.Body.String(), `"helpful_ask":1`) {
		t.Fatalf("tick %s", rec.Body.String())
	}
	if rec := tick(t, h, "tick-secret"); !strings.Contains(rec.Body.String(), `"helpful_ask":0`) {
		t.Fatalf("tick again %s", rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/helpful", tu, map[string]any{"helpful": "maybe"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad helpful %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/helpful", tu, map[string]any{"helpful": "yes"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"helpful":"yes"`) || !strings.Contains(rec.Body.String(), `"helpful_asked":true`) {
		t.Fatalf("helpful %d %s", rec.Code, rec.Body.String())
	}
}

func TestReviewResponseReportAndProbation(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, tadmin)
	_, tu := demoRequester(t, h, tadmin)
	id := acceptedReview(t, h, tadmin, tc, tu)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, responseBody()); rec.Code != http.StatusOK {
		t.Fatalf("response %d", rec.Code)
	}

	report := map[string]any{"target_type": "review_response", "target_id": id, "reason": "harassment", "detail": ""}
	// 通報できるのは依頼した本人だけ。
	if rec := doJSON(t, h, http.MethodPost, "/v1/reports", token(t, uuid.New()), report); rec.Code != http.StatusNotFound {
		t.Fatalf("other report %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/reports", tu, report); rec.Code != http.StatusCreated {
		t.Fatalf("report %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/admin/reports", tadmin, nil)
	if !strings.Contains(rec.Body.String(), `"target_type":"review_response"`) || !strings.Contains(rec.Body.String(), "1台目までの8歩") {
		t.Fatalf("admin reports %s", rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/coaching/probation", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), coach.String()) || !strings.Contains(rec.Body.String(), `"nth":1`) {
		t.Fatalf("probation %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/coaching/probation", tu, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("probation non-admin %d", rec.Code)
	}

	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/responses/"+id+"/hide", tadmin, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("hide %d", rec.Code)
	}
	body := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+id, tu, nil).Body.String()
	if !strings.Contains(body, `"hidden":true`) || strings.Contains(body, "1台目までの8歩") {
		t.Fatalf("hidden response shown %s", body)
	}
}

func TestLowRatedCoachSkipped(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupConfig(t, func(cfg *config.Config) { cfg.CoachingTickSecret = "tick-secret" })
	tadmin := adminToken(t, h)
	aID, ta := onDutyCoach(t, h, tadmin)
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", ta, map[string]any{"daily_cap": 10}); rec.Code != http.StatusOK {
		t.Fatalf("daily cap %d", rec.Code)
	}

	// ★1 を1件受けただけでは外れない。
	_, tu1 := demoRequester(t, h, tadmin)
	id := acceptedReview(t, h, tadmin, ta, tu1)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", ta, responseBody()); rec.Code != http.StatusOK {
		t.Fatalf("response %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/rating", tu1, map[string]any{"stars": 1}); rec.Code != http.StatusOK {
		t.Fatalf("rating %d", rec.Code)
	}
	if body := doJSON(t, h, http.MethodGet, "/v1/coach/me", ta, nil).Body.String(); !strings.Contains(body, `"paused":false`) {
		t.Fatalf("paused too early %s", body)
	}
	// 同じような返却済みの依頼をあと4件足し、★1 を合わせて5件にする。
	for i := 0; i < 4; i++ {
		copied := uuid.New()
		execSQLArgs(t, `INSERT INTO review_requests (id, requester_id, tier, event, focus, video_public_id, status, due_at, coach_id)
			SELECT $1::uuid, requester_id, tier, event, focus, video_public_id || $1::text, 'returned', due_at, coach_id FROM review_requests WHERE id = $2`, copied, id)
		execSQLArgs(t, `INSERT INTO review_responses (request_id, coach_id, good_point, fix_points, drill)
			SELECT $1, coach_id, good_point, fix_points, drill FROM review_responses WHERE request_id = $2`, copied, id)
		execSQLArgs(t, `INSERT INTO review_ratings (request_id, stars, rated_at) VALUES ($1, 1, now())`, copied)
	}
	if body := doJSON(t, h, http.MethodGet, "/v1/coach/me", ta, nil).Body.String(); !strings.Contains(body, `"paused":true`) {
		t.Fatalf("not paused %s", body)
	}

	// 平均が★3未満になったので、受付中でも打診されない。
	_, tu := demoRequester(t, h, tadmin)
	if body := doJSON(t, h, http.MethodGet, "/v1/coaching/estimate?tier=coach&event=100m", tu, nil).Body.String(); !strings.Contains(body, `"coaches_on_duty":false`) {
		t.Fatalf("estimate %s", body)
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBodyFor(t, h, tu))
	var created reviewRequestView
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if rec.Code != http.StatusCreated || created.Status != "queued" {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	if len(coachRequests(t, h, ta).Reviewing) != 0 {
		t.Fatal("low-rated coach got an offer")
	}

	// 運営の画面では外れていることがわかり、戻すと待っていた依頼が回る。
	if body := doJSON(t, h, http.MethodGet, "/v1/admin/coach-applications?status=approved", tadmin, nil).Body.String(); !strings.Contains(body, `"rating_paused":true`) {
		t.Fatalf("admin list %s", body)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coaches/"+uuid.NewString()+"/reset-rating", tadmin, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("reset unknown %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coaches/"+aID.String()+"/reset-rating", ta, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("reset by coach %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coaches/"+aID.String()+"/reset-rating", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("reset %d %s", rec.Code, rec.Body.String())
	}
	if body := doJSON(t, h, http.MethodGet, "/v1/coach/me", ta, nil).Body.String(); !strings.Contains(body, `"paused":false`) {
		t.Fatalf("still paused %s", body)
	}
	if offers := coachRequests(t, h, ta).Reviewing; len(offers) != 1 || offers[0].ID != created.ID {
		t.Fatalf("A offers after reset %+v", offers)
	}
}

// Keep authorization probes separate from the long happy-path flow so rate limits
// cannot mask either the authorization checks or the legitimate coach operations.
func TestReviewAuthorization(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupConfig(t, nil)
	tadmin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, tadmin)
	_, tother := onDutyCoach(t, h, tadmin)
	_, tu := demoRequester(t, h, tadmin)
	id := acceptedReview(t, h, tadmin, tc, tu)
	// These routes must reject unauthenticated callers before inspecting request data.
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/v1/coach/requests"},
		{http.MethodGet, "/v1/coach/requests/" + id},
		{http.MethodPost, "/v1/coaching/requests/" + id + "/response"},
		{http.MethodPost, "/v1/coach/requests/" + id + "/start"},
		{http.MethodPost, "/v1/coach/requests/" + id + "/withdraw"},
	} {
		if rec := doJSON(t, h, route.method, route.path, "", nil); rec.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s: %d", route.path, rec.Code)
		}
	}
	for _, unauthorized := range []string{tother, tu} {
		want := http.StatusNotFound
		if unauthorized == tu {
			want = http.StatusForbidden
		}
		if rec := doJSON(t, h, http.MethodGet, "/v1/coach/requests/"+id, unauthorized, nil); rec.Code != want {
			t.Fatalf("unauthorized request read %d %s", rec.Code, rec.Body.String())
		}
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tu, responseBody()); rec.Code != http.StatusForbidden {
		t.Fatalf("requester submits response %d", rec.Code)
	}

	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, responseBody()); rec.Code != http.StatusOK {
		t.Fatalf("coach response %d %s", rec.Code, rec.Body.String())
	}
}

// Approval must be checked on every request, even when an assignment still
// belongs to the caller and their authentication token remains valid.
func TestReviewRevokedCoachAuthorization(t *testing.T) {
	for _, operation := range []string{"list_offer", "list_review", "read", "response", "decline"} {
		t.Run(operation, func(t *testing.T) {
			fakeCoachingCloudinary(t)
			h := setup(t)
			tadmin := adminToken(t, h)
			coach, tc := onDutyCoach(t, h, tadmin)
			_, tu := demoRequester(t, h, tadmin)
			rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBodyFor(t, h, tu))
			if rec.Code != http.StatusCreated {
				t.Fatalf("create %d %s", rec.Code, rec.Body.String())
			}
			offers := coachRequests(t, h, tc).Reviewing
			if len(offers) != 1 {
				t.Fatalf("offers %+v", offers)
			}
			offer := offers[0]
			id := offer.ID
			if operation != "list_offer" && operation != "decline" {
				if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+offer.ID+"/start", tc, nil); rec.Code != http.StatusOK {
					t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
				}
			}
			execSQLArgs(t, `UPDATE coach_profiles SET status = 'suspended', on_duty = false WHERE profile_id = $1`, coach)
			before := reviewStatus(t, h, tu, id)
			method, path := http.MethodGet, "/v1/coach/requests"
			var body any
			switch operation {
			case "read":
				path += "/" + id
			case "response":
				method, path, body = http.MethodPost, "/v1/coaching/requests/"+id+"/response", responseBody()
			case "decline":
				method, path = http.MethodPost, "/v1/coach/requests/"+offer.ID+"/withdraw"
			}
			rec = doJSON(t, h, method, path, tc, body)
			if rec.Code != http.StatusForbidden {
				t.Errorf("suspended coach: got %d, want 403", rec.Code)
			}
			if after := reviewStatus(t, h, tu, id); after != before {
				t.Errorf("suspended coach changed request: %s -> %s", before, after)
			}
		})
	}
}
