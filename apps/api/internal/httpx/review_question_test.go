package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/migrations"
	"github.com/jackc/pgx/v5"
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

func TestReviewWithoutVideo(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, stripe := setupCoachChoice(t)
	admin := adminToken(t, h)
	coachID, coach := onDutyCoach(t, h, admin)
	ownerID, owner := demoRequester(t, h, admin)
	buySixTickets(t, h, owner)
	addRewardLot(t, ownerID, time.Now().Add(30*24*time.Hour))
	const route = "/v1/coaching/requests"
	body := func() map[string]any {
		return map[string]any{
			"tier": "coach", "event": "100m", "method": "card",
			"requested_coach_id": coachID, "guardian_permission": true,
		}
	}
	for _, input := range []map[string]any{
		{"focus": []string{"start"}},
		{"video_public_id": "", "question": "  "},
		{"video_public_id": "not-a-coaching-video", "question": "大会前の練習について"},
		// 申込の動画は必須（#1036）。相談内容があっても、動画なしでは新しく作れない。
		{"question": "大会前の練習について"},
		{"video_public_id": "", "question": "大会前の練習について"},
		{"question": "大会前の練習について", "method": "tickets", "ticket_kind": "normal"},
		{"question": "大会前の練習について", "method": "tickets", "ticket_kind": "reward"},
		{"question": "大会前の練習について", "use_coupon": true},
	} {
		request := body()
		field := "question"
		for key, value := range input {
			request[key] = value
			if key == "video_public_id" && value == "not-a-coaching-video" {
				field = key
			}
		}
		if _, ok := input["question"]; ok && strings.TrimSpace(input["question"].(string)) != "" {
			field = "video_public_id"
		}
		rec := doJSON(t, h, http.MethodPost, route, owner, request)
		if rec.Code != 422 || !strings.Contains(rec.Body.String(), `"`+field+`"`) {
			t.Fatalf("invalid request: %d %s", rec.Code, rec.Body.String())
		}
	}

	for _, payment := range []string{"card", "normal", "reward"} {
		for _, expire := range []bool{true, false} {
			request := body()
			request["question"] = "大会前の練習について相談したいです"
			request["video_public_id"] = signedCoachingVideo(t, h, owner)
			if payment != "card" {
				request["method"], request["ticket_kind"] = "tickets", payment
			}
			rec := doJSON(t, h, http.MethodPost, route, owner, request)
			if rec.Code != http.StatusCreated {
				t.Fatalf("%s create: %d %s", payment, rec.Code, rec.Body.String())
			}
			var created struct{ ID string }
			if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
				t.Fatal(err)
			}
			id := created.ID
			// 動画なしの依頼は新しく作れなくなった（#1036）。必須化より前に作られた動画なしの依頼が、
			// 支払い・期限切れ・返答をこれまでどおり通れることを確かめるため、作成後に動画を外して再現する。
			execSQLArgs(t, `UPDATE review_requests SET video_public_id=NULL WHERE id=$1`, id)
			if payment == "card" {
				payChosenReview(t, h, id, "card")
			}
			detail := doJSON(t, h, http.MethodGet, route+"/"+id, owner, nil)
			if detail.Code != 200 || !strings.Contains(detail.Body.String(), `"video_url":null`) || !strings.Contains(detail.Body.String(), `"status":"in_review"`) {
				t.Fatalf("%s detail: %d %s", payment, detail.Code, detail.Body.String())
			}
			if expire {
				execSQLArgs(t, `UPDATE review_requests SET due_at=now()-interval '1 minute' WHERE id=$1`, id)
				if payment == "card" {
					execSQLArgs(t, `UPDATE review_payments SET authorized_at=now()-interval '2 minutes' WHERE request_id=$1`, id)
				}
				if rec := tick(t, h, "tick-secret"); rec.Code != 200 {
					t.Fatal(rec.Body.String())
				}
				if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", coach, map[string]any{"on_duty": true}); rec.Code != 200 {
					t.Fatalf("resume accepting requests: %d %s", rec.Code, rec.Body.String())
				}
			} else {
				if rec := doJSON(t, h, "POST", "/v1/coach/requests/"+id+"/start", coach, nil); rec.Code != 200 {
					t.Fatal(rec.Body.String())
				}
				if rec := doJSON(t, h, "POST", route+"/"+id+"/response", coach, responseBody()); rec.Code != 200 {
					t.Fatal(rec.Body.String())
				}
			}
			if payment == "card" {
				captured, canceled := stripe.calls()
				if (expire && len(canceled) != 1) || (!expire && len(captured) != 1) {
					t.Fatalf("card capture=%v cancel=%v", captured, canceled)
				}
				continue
			}
			var status string
			queryRowArgs(t, `SELECT status FROM coaching_ticket_reservations WHERE request_id=$1`, []any{id}, &status)
			want := "consumed"
			if expire {
				want = "released"
			}
			if status != want {
				t.Fatalf("%s ticket %q, want %q", payment, status, want)
			}
		}
	}
}

func TestReviewWithoutVideoResubmit(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coachID, coach := onDutyCoach(t, h, admin)
	for _, addVideo := range []bool{false, true} {
		_, owner := demoRequester(t, h, admin)
		const route = "/v1/coaching/requests"
		rec := doJSON(t, h, http.MethodPost, route, owner, map[string]any{
			"tier": "coach", "event": "100m", "method": "card", "question": "大会前の練習について",
			"requested_coach_id": coachID, "guardian_permission": true,
			"video_public_id": signedCoachingVideo(t, h, owner),
		})
		if rec.Code != 201 {
			t.Fatal(rec.Body.String())
		}
		var created struct{ ID string }
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		id := created.ID
		// 必須化（#1036）より前に作られた動画なしの依頼を再現する。
		execSQLArgs(t, `UPDATE review_requests SET video_public_id=NULL WHERE id=$1`, id)
		payChosenReview(t, h, id, "card")
		askInfo(t, h, coach, id, map[string]any{"body": "大会の日程と、現在の練習内容を詳しく教えてください。"}, http.StatusOK)
		body := map[string]any{"question": "大会は来週で、練習は週に三回です"}
		if addVideo {
			body["video_public_id"] = signedCoachingVideo(t, h, owner)
		}
		rec = doJSON(t, h, http.MethodPost, route+"/"+id+"/resubmit", owner, body)
		if rec.Code != 200 {
			t.Fatal(rec.Body.String())
		}
		view := requesterReviewView(t, h, owner, id)
		if (view.VideoURL != "") != addVideo || view.InfoRequest == nil || view.InfoRequest.VideoReplaced != addVideo {
			t.Fatalf("add video %t: %+v", addVideo, view)
		}
	}
}

func TestReviewOptionalVideoMigrationRoundTrip(t *testing.T) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	exec := func(sql string) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	migration := func(direction string) string {
		t.Helper()
		sql, err := migrations.FS.ReadFile("000173_review_request_optional_video." + direction + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		return string(sql)
	}
	exec(`CREATE TEMP TABLE review_requests (video_public_id text NOT NULL UNIQUE)`)
	exec(`INSERT INTO review_requests VALUES ('existing-video')`)
	exec(migration("up"))
	exec(`INSERT INTO review_requests VALUES (NULL), (NULL)`)
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM review_requests WHERE video_public_id = 'existing-video'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("existing video count=%d err=%v", count, err)
	}
	exec(`SAVEPOINT duplicate_video`)
	if _, err := tx.Exec(ctx, `INSERT INTO review_requests VALUES ('existing-video')`); err == nil {
		t.Fatal("duplicate video accepted")
	}
	exec(`ROLLBACK TO SAVEPOINT duplicate_video`)
	exec(`SAVEPOINT rollback_with_nulls`)
	if _, err := tx.Exec(ctx, migration("down")); err == nil {
		t.Fatal("down unexpectedly allowed NULL videos")
	}
	exec(`ROLLBACK TO SAVEPOINT rollback_with_nulls`)
	exec(`DELETE FROM review_requests WHERE video_public_id IS NULL`)
	exec(migration("down"))
	exec(`SAVEPOINT confirm_not_null`)
	if _, err := tx.Exec(ctx, `INSERT INTO review_requests VALUES (NULL)`); err == nil {
		t.Fatal("down did not restore NOT NULL")
	}
	exec(`ROLLBACK TO SAVEPOINT confirm_not_null`)
	exec(migration("up"))
	exec(`INSERT INTO review_requests VALUES (NULL)`)
}

// 申込動画の上限は60秒（#1036）。45秒・60秒は作れて、61秒は「動画は60秒以内にしてください」で断る。
func TestReviewVideoUpToSixtySeconds(t *testing.T) {
	duration := "45"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/resources/video/authenticated/track-talk-coaching/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resource_type":"video","type":"authenticated","format":"mp4","bytes":1024,"duration":` + duration + `}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("CLOUDINARY_CLOUD_NAME", "demo")
	t.Setenv("CLOUDINARY_API_KEY", "key")
	t.Setenv("CLOUDINARY_API_SECRET", "secret")
	t.Setenv("CLOUDINARY_API_BASE", srv.URL)
	h := setup(t)
	admin := adminToken(t, h)
	onDutyCoach(t, h, admin)
	_, owner := demoRequester(t, h, admin)
	for _, d := range []string{"45", "60"} {
		duration = d
		if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", owner, reviewBodyFor(t, h, owner)); rec.Code != http.StatusCreated {
			t.Fatalf("%s seconds: %d %s", d, rec.Code, rec.Body.String())
		}
	}
	duration = "61"
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", owner, reviewBodyFor(t, h, owner))
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "動画は60秒以内にしてください") {
		t.Fatalf("61 seconds: %d %s", rec.Code, rec.Body.String())
	}
}
