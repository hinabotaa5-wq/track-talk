package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// 相談内容（作成・再提出）にも、他の接点と同じ連絡先の判定をかける（本体 #485）。

const contactMessage = "連絡先らしきものは書けません"

// 連絡先の形（URL・電話・メール・SNS の ID）。パターンは contact_test.go の既存ケースに揃える。
var contactQuestions = map[string]string{
	"url":   "詳しくは https://example.com を見てください",
	"phone": "090-1234-5678 に電話してください",
	"email": "coach@example.com に送ってください",
	"sns":   "インスタの @track_coach をフォローしてください",
}

func countReviewRequests(t *testing.T, requester any) int {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*) FROM review_requests WHERE requester_id=$1`, []any{requester}, &n)
	return n
}

func TestReviewQuestionRejectsContactOnCreate(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	admin := adminToken(t, h)
	onDutyCoach(t, h, admin)
	ownerID, owner := demoRequester(t, h, admin)

	for kind, question := range contactQuestions {
		body := reviewBodyFor(t, h, owner)
		delete(body, "focus")
		body["question"] = question
		rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", owner, body)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"question"`) ||
			!strings.Contains(rec.Body.String(), contactMessage) {
			t.Fatalf("%s: contact question accepted: %d %s", kind, rec.Code, rec.Body.String())
		}
		if n := countReviewRequests(t, ownerID); n != 0 {
			t.Fatalf("%s: request saved after rejection: %d", kind, n)
		}
	}

	// 既存の検証（空・1000文字超）は従来どおりの文言で止まる。
	for question, want := range map[string]string{
		"  ":                          "相談内容を記入してください",
		strings.Repeat("走", 1001):     "1000文字以内で記入してください",
		strings.Repeat("a@b.jp", 200): "1000文字以内で記入してください",
	} {
		body := reviewBodyFor(t, h, owner)
		delete(body, "focus")
		body["question"] = question
		rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", owner, body)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("existing validation changed: %d %s", rec.Code, rec.Body.String())
		}
	}

	// 連絡先を含まない相談内容は作成できる。
	body := reviewBodyFor(t, h, owner)
	delete(body, "focus")
	body["question"] = "100mの後半に失速します。スタートラインからの加速を見てください。"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", owner, body); rec.Code != http.StatusCreated {
		t.Fatalf("plain question: %d %s", rec.Code, rec.Body.String())
	}
	// 相談内容なしで「見てほしい点」だけの経路も従来どおり。
	_, other := demoRequester(t, h, admin)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", other, reviewBodyFor(t, h, other)); rec.Code != http.StatusCreated {
		t.Fatalf("focus only: %d %s", rec.Code, rec.Body.String())
	}
}

func TestReviewQuestionRejectsContactOnResubmit(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coachID, coach := onDutyCoach(t, h, admin)
	_, owner := demoRequester(t, h, admin)
	const route = "/v1/coaching/requests"
	const original = "大会前の練習について"
	rec := doJSON(t, h, http.MethodPost, route, owner, map[string]any{
		"tier": "coach", "event": "100m", "method": "card", "question": original,
		"requested_coach_id": coachID, "guardian_permission": true,
		"video_public_id": signedCoachingVideo(t, h, owner),
	})
	if rec.Code != http.StatusCreated {
		t.Fatal(rec.Body.String())
	}
	var created struct{ ID string }
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id := created.ID
	payChosenReview(t, h, id, "card")
	askInfo(t, h, coach, id, map[string]any{"body": infoQuestion}, http.StatusOK)

	for kind, question := range contactQuestions {
		rec := doJSON(t, h, http.MethodPost, route+"/"+id+"/resubmit", owner, map[string]any{"question": question})
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"question"`) ||
			!strings.Contains(rec.Body.String(), contactMessage) {
			t.Fatalf("%s: contact resubmit accepted: %d %s", kind, rec.Code, rec.Body.String())
		}
		view := requesterReviewView(t, h, owner, id)
		if view.Status != "needs_info" || view.Question != original || view.InfoRequest == nil || view.InfoRequest.ResubmittedAt != nil {
			t.Fatalf("%s: request changed after rejection: %+v", kind, view)
		}
	}

	// 既存の検証（空・1000文字超・未変更）は従来どおりの文言で止まる。
	for question, want := range map[string]string{
		"  ":                      "相談内容を記入してください",
		strings.Repeat("走", 1001): "1000文字以内で記入してください",
		original:                  "コーチの質問に答えるように",
	} {
		rec := doJSON(t, h, http.MethodPost, route+"/"+id+"/resubmit", owner, map[string]any{"question": question})
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("existing validation changed: %d %s", rec.Code, rec.Body.String())
		}
	}

	// 連絡先を含まない修正文は再提出できる。
	fixed := "60m以降の後半です。失速しないためのポイントを教えてください。"
	rec = doJSON(t, h, http.MethodPost, route+"/"+id+"/resubmit", owner, map[string]any{"question": fixed})
	if rec.Code != http.StatusOK {
		t.Fatalf("plain resubmit: %d %s", rec.Code, rec.Body.String())
	}
	if view := requesterReviewView(t, h, owner, id); view.Question != fixed || view.Status == "needs_info" {
		t.Fatalf("resubmitted: %+v", view)
	}
}
