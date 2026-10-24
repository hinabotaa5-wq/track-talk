package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestReviewQuestionIsValidatedAndReachesCoach(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	admin := adminToken(t, h)
	_, coach := onDutyCoach(t, h, admin)
	_, owner := demoRequester(t, h, admin)
	for _, question := range []string{"", " \n ", strings.Repeat("走", 1001)} {
		body := reviewBodyFor(t, h, owner)
		delete(body, "focus")
		body["question"] = question
		response := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", owner, body)
		if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), `"question"`) {
			t.Fatalf("invalid question accepted: %d %s", response.Code, response.Body.String())
		}
	}
	question := "後半に腰が落ちます。\n改善点と練習方法を教えてください。"
	body := reviewBodyFor(t, h, owner)
	delete(body, "focus")
	body["question"] = "  " + question + "  "
	response := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", owner, body)
	if response.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", response.Code, response.Body.String())
	}
	var created struct {
		ID       string
		Question string
		Focus    []string
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Question != question || len(created.Focus) != 0 {
		t.Fatalf("saved question: %+v", created)
	}
	detail := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+created.ID, owner, nil)
	var read struct{ Question string }
	if err := json.Unmarshal(detail.Body.Bytes(), &read); err != nil {
		t.Fatal(err)
	}
	if detail.Code != http.StatusOK || read.Question != question {
		t.Fatalf("detail: %d %s", detail.Code, detail.Body.String())
	}
	offers := doJSON(t, h, http.MethodGet, "/v1/coach/requests", coach, nil)
	encoded, _ := json.Marshal(question)
	if offers.Code != http.StatusOK || !strings.Contains(offers.Body.String(), string(encoded)) {
		t.Fatalf("coach cannot read question: %d %s", offers.Code, offers.Body.String())
	}
}
