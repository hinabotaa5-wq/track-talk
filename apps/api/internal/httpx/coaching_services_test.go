package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCoachingServicesAndPublication(t *testing.T) {
	h := setup(t)
	_, tc, planID := planCoach(t, h, 3)
	path := "/v1/coach/me/services"
	for _, body := range []map[string]any{{}, {"service_types": nil}, {"service_types": []string{"unknown"}}} {
		if rec := doJSON(t, h, http.MethodPatch, path, tc, body); rec.Code != 422 {
			t.Fatalf("invalid services %d %s", rec.Code, rec.Body.String())
		}
	}
	if rec := doJSON(t, h, http.MethodPatch, path, token(t, uuid.New()), map[string]any{"service_types": []string{"video_review"}}); rec.Code != 403 {
		t.Fatalf("not a coach %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodPatch, path, tc, map[string]any{"service_types": []string{"in_person", "video_review", "competition_support", "video_review"}})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"service_types":["video_review","competition_support","in_person"]`) {
		t.Fatalf("services %d %s", rec.Code, rec.Body.String())
	}
	for _, service := range []string{"competition_support", "in_person"} {
		body := planBody()
		body["service_type"] = service
		if rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, body); rec.Code != 422 {
			t.Fatalf("premature publication %d %s", rec.Code, rec.Body.String())
		}
		body["published"] = false
		rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, body)
		if rec.Code != 201 || !strings.Contains(rec.Body.String(), `"service_type":"`+service+`"`) {
			t.Fatalf("draft %d %s", rec.Code, rec.Body.String())
		}
		var result struct{ Plan struct{ ID string } }
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if rec := doJSON(t, h, http.MethodGet, "/v1/coaching/plans/"+result.Plan.ID, "", nil); rec.Code != 404 {
			t.Fatalf("public draft %d %s", rec.Code, rec.Body.String())
		}
	}
	body := planBody()
	body["service_type"] = "video_review"
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+planID, tc, body); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPatch, path, tc, map[string]any{"service_types": []string{}}); rec.Code != 409 {
		t.Fatalf("remove published service %d %s", rec.Code, rec.Body.String())
	}
	body["published"] = false
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+planID, tc, body); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPatch, path, tc, map[string]any{"service_types": []string{}}); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	body["published"] = true
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+planID, tc, body); rec.Code != 422 {
		t.Fatalf("unhandled service %d %s", rec.Code, rec.Body.String())
	}
	// 古いプランは、分類を推測したり既存の販売条件を変えたりしない。
	legacy := planBody()
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, legacy)
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), `"service_type":null`) {
		t.Fatal(rec.Body.String())
	}
}

func TestCoachingServiceTermsSnapshot(t *testing.T) {
	h := setup(t)
	_, tc, planID := planCoach(t, h, 3)
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/services", tc, map[string]any{"service_types": []string{"video_review"}}); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	body := planBody()
	body["service_type"] = "video_review"
	body["additional_consultation_price_yen"] = 500
	for _, bad := range []any{499, 10000000, 500.5, "500"} {
		body["additional_consultation_price_yen"] = bad
		if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+planID, tc, body); rec.Code != 422 {
			t.Fatalf("invalid additional price %v: %d %s", bad, rec.Code, rec.Body.String())
		}
	}
	body["additional_consultation_price_yen"] = 500
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+planID, tc, body); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu, contractBody(planID))
	if rec.Code != 201 {
		t.Fatal(rec.Body.String())
	}
	ct := decodeContract(t, rec.Body.Bytes())
	oldDescription := body["description"].(string)
	body["description"] = "新しい内容"
	body["additional_consultation_price_yen"] = 1000
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+planID, tc, body); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	// コーチ・利用者の詳細と一覧で、申込時点の内容と追加料金を返す。
	for _, check := range []struct{ path, tok string }{
		{"/v1/coaching/contracts/" + ct.ID, tu},
		{"/v1/coaching/contracts/" + ct.ID, tc},
		{"/v1/me/coaching/contracts", tu},
		{"/v1/coach/contracts", tc},
	} {
		rec := doJSON(t, h, http.MethodGet, check.path, check.tok, nil)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"service_type":"video_review"`) || !strings.Contains(rec.Body.String(), `"additional_consultation_price_yen":500`) || !strings.Contains(rec.Body.String(), `"plan_description":"`+oldDescription+`"`) {
			t.Fatalf("snapshot %s %d %s", check.path, rec.Code, rec.Body.String())
		}
	}
	// 古いクライアントの保存では新しい設定を消さない。
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+planID, tc, planBody()); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"additional_consultation_price_yen":1000`) || !strings.Contains(rec.Body.String(), `"service_type":"video_review"`) {
		t.Fatal(rec.Body.String())
	}
	body["additional_consultation_price_yen"] = nil
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+planID, tc, body); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"additional_consultation_price_yen":null`) {
		t.Fatal(rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil)
	if !strings.Contains(rec.Body.String(), `"service_types":["video_review"]`) || !strings.Contains(rec.Body.String(), `"service_type":"video_review"`) {
		t.Fatal(rec.Body.String())
	}
}

func TestCoachingServiceRemovalAndPublicationAreSerialized(t *testing.T) {
	h := setup(t)
	_, tc, _ := planCoach(t, h, 3)
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/services", tc, map[string]any{"service_types": []string{"video_review"}}); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var wg sync.WaitGroup
	codes := make([]int, 2)
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		body := planBody()
		body["service_type"] = "video_review"
		codes[0] = doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, body).Code
	}()
	go func() {
		defer wg.Done()
		<-start
		codes[1] = doJSON(t, h, http.MethodPatch, "/v1/coach/me/services", tc, map[string]any{"service_types": []string{}}).Code
	}()
	close(start)
	wg.Wait()
	if !((codes[0] == 201 && codes[1] == 409) || (codes[0] == 422 && codes[1] == 200)) {
		t.Fatalf("publication and removal must not both succeed: %v", codes)
	}
}
