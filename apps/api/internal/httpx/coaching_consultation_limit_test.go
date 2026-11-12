package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// answeredCashReviewRoom は、チケットを持たない利用者の、返却済みの動画添削の部屋を作る（追加相談はその場で払う）。
func answeredCashReviewRoom(t *testing.T, mutate func(*config.Config)) (h http.Handler, f *fakeStripe, owner, coach, root string) {
	t.Helper()
	fakeCoachingCloudinary(t)
	h, f = setupStripeConfig(t, func(c *config.Config) {
		c.CoachingTicketSales = false
		if mutate != nil {
			mutate(c)
		}
	})
	admin := adminToken(t, h)
	_, coach = onDutyCoach(t, h, admin)
	_, owner = demoRequester(t, h, admin)
	root = paidReview(t, h, owner, "paypay", "pi_paypay_review_"+uuid.NewString())
	offers := coachRequests(t, h, coach).Reviewing
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if r := doJSON(t, h, "POST", "/v1/coach/requests/"+offers[0].ID+"/start", coach, nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+root+"/response", coach, responseBody()); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	return
}

type roomState struct {
	CanPurchase bool   `json:"can_purchase"`
	BlockReason string `json:"block_reason"`
}

func reviewRoomState(t *testing.T, h http.Handler, tok, root string) roomState {
	t.Helper()
	r := doJSON(t, h, "GET", "/v1/coaching/requests/"+root+"/consultations", tok, nil)
	if r.Code != 200 {
		t.Fatalf("room %d %s", r.Code, r.Body.String())
	}
	var out roomState
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// 支払い前の取り下げ・期限切れは試行に数えない。回答がないまま終わったときだけ、もう1回だけ買い直せる。
func TestReviewConsultationRebuyOnlyOnceAfterNoAnswer(t *testing.T) {
	h, _, owner, coach, root := answeredCashReviewRoom(t, nil)
	path := "/v1/coaching/requests/" + root + "/consultations"

	// 支払い前に取り下げたもの。
	c1 := consultationDraft(t, h, owner, path, uuid.NewString(), "取り下げる質問")
	consultationAction(t, h, owner, c1.ID, "submit", nil)
	consultationAction(t, h, owner, c1.ID, "cancel", nil)
	// 支払わないまま期限が過ぎたもの。
	c2 := consultationDraft(t, h, owner, path, uuid.NewString(), "払わない質問")
	consultationAction(t, h, owner, c2.ID, "submit", nil)
	execSQLArgs(t, `UPDATE coaching_consultations SET expires_at=now()-interval '1 minute' WHERE id=$1`, c2.ID)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_consultations WHERE id=$1`, []any{c2.ID}, &status)
	if status != "expired" {
		t.Fatal(status)
	}
	// 支払いが期限の後に届いて返金で終わったもの（コーチには届いていない）。
	c3 := consultationDraft(t, h, owner, path, uuid.NewString(), "遅れて払った質問")
	consultationAction(t, h, owner, c3.ID, "submit", nil)
	execSQLArgs(t, `UPDATE coaching_consultations SET expires_at=now()-interval '1 minute' WHERE id=$1`, c3.ID)
	if code := payConsultation(t, h, c3.ID, 600); code != 200 {
		t.Fatalf("late pay %d", code)
	}
	queryRowArgs(t, `SELECT status FROM coaching_consultations WHERE id=$1`, []any{c3.ID}, &status)
	if status != "expired" {
		t.Fatal(status)
	}
	if s := reviewRoomState(t, h, owner, root); !s.CanPurchase {
		t.Fatalf("unpaid attempts counted %+v", s)
	}

	// 1回目：コーチが辞退した。回答がないので、もう1回だけ買い直せる。
	first := consultationDraft(t, h, owner, path, uuid.NewString(), "1回目の質問")
	consultationAction(t, h, owner, first.ID, "submit", nil)
	if code := payConsultation(t, h, first.ID, 600); code != 200 {
		t.Fatalf("pay %d", code)
	}
	consultationAction(t, h, coach, first.ID, "decline", nil)
	if s := reviewRoomState(t, h, owner, root); !s.CanPurchase {
		t.Fatalf("rebuy after decline %+v", s)
	}

	// 2回目（買い直し）：引き受けた後、回答の期限が過ぎた。買い直しは1回だけなので、もう買えない。
	second := consultationDraft(t, h, owner, path, uuid.NewString(), "買い直した質問")
	consultationAction(t, h, owner, second.ID, "submit", nil)
	if code := payConsultation(t, h, second.ID, 600); code != 200 {
		t.Fatalf("pay %d", code)
	}
	consultationAction(t, h, coach, second.ID, "accept", nil)
	execSQLArgs(t, `UPDATE coaching_consultations SET answer_by=now()-interval '1 minute' WHERE id=$1`, second.ID)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	queryRowArgs(t, `SELECT status FROM coaching_consultations WHERE id=$1`, []any{second.ID}, &status)
	if status != "expired" {
		t.Fatal(status)
	}
	if s := reviewRoomState(t, h, owner, root); s.CanPurchase || !strings.Contains(s.BlockReason, "上限") {
		t.Fatalf("third purchase allowed %+v", s)
	}
	if r := doJSON(t, h, "POST", path, owner, map[string]any{"request_key": uuid.NewString(), "body": "3回目"}); r.Code != 422 || !strings.Contains(r.Body.String(), "consultation_limit") {
		t.Fatalf("third draft %d %s", r.Code, r.Body.String())
	}
}

// 上限より前に保存した下書きでも、送るときに同じ判定で止める（画面だけで塞がない）。
func TestReviewConsultationLimitAppliesOnSubmit(t *testing.T) {
	h, _, owner, coach, root := answeredCashReviewRoom(t, nil)
	path := "/v1/coaching/requests/" + root + "/consultations"
	first := consultationDraft(t, h, owner, path, uuid.NewString(), "回答をもらう質問")
	consultationAction(t, h, owner, first.ID, "submit", nil)
	if code := payConsultation(t, h, first.ID, 600); code != 200 {
		t.Fatalf("pay %d", code)
	}
	consultationAction(t, h, coach, first.ID, "accept", nil)
	consultationAction(t, h, coach, first.ID, "answer", map[string]any{"body": "回答です"})
	// 回答の前から残っていた下書きを再現する。
	draft := uuid.NewString()
	execSQLArgs(t, `INSERT INTO coaching_consultations (id,review_request_id,requester_id,coach_id,request_key,minor,funding,price_yen,share_percent,question)
SELECT $1,review_request_id,requester_id,coach_id,gen_random_uuid(),minor,'cash',600,share_percent,'古い下書き' FROM coaching_consultations WHERE id=$2`, draft, first.ID)
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+draft+"/submit", owner, nil); r.Code != 422 || !strings.Contains(r.Body.String(), "consultation_limit") {
		t.Fatalf("submit %d %s", r.Code, r.Body.String())
	}
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_consultations WHERE id=$1`, []any{draft}, &status)
	if status != "draft" {
		t.Fatal(status)
	}
}

// 新しいゲートが閉じている間は、動画添削のあとの追加相談の送信・支払い・回答だけを止める。
// 下書き・閲覧・取り下げは止めず、サポートプランの追加相談は COACHING_MESSAGING_ENABLED のままで動く。
func TestReviewConsultationGateClosed(t *testing.T) {
	h, _, owner, coach, root := answeredCashReviewRoom(t, func(c *config.Config) { c.CoachingConsultation = false })
	path := "/v1/coaching/requests/" + root + "/consultations"
	if r := doJSON(t, h, "GET", path, owner, nil); r.Code != 200 {
		t.Fatalf("read %d %s", r.Code, r.Body.String())
	}
	c := consultationDraft(t, h, owner, path, uuid.NewString(), "ゲートが閉じている間の下書き")
	for _, p := range []string{"/submit", "/checkout"} {
		r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+p, owner, nil)
		if r.Code != http.StatusConflict || !strings.Contains(r.Body.String(), "consultation_closed_for_now") {
			t.Fatalf("%s %d %s", p, r.Code, r.Body.String())
		}
	}
	// 届いた後の回答も止める（ゲートを閉じる前に届いていた相談を再現する）。
	execSQLArgs(t, `UPDATE coaching_consultations SET status='in_review',respond_by=now(),answer_by=now()+interval '1 day' WHERE id=$1`, c.ID)
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/answer", coach, map[string]any{"body": "回答"}); r.Code != http.StatusConflict || !strings.Contains(r.Body.String(), "consultation_closed_for_now") {
		t.Fatalf("answer %d %s", r.Code, r.Body.String())
	}
	execSQLArgs(t, `UPDATE coaching_consultations SET status='draft',respond_by=NULL,answer_by=NULL WHERE id=$1`, c.ID)
	consultationAction(t, h, owner, c.ID, "cancel", nil)

	// サポートプランの追加相談は、やりとりのゲートが開いていれば送れて、回答への確認もできる。
	contractOwner, contractCoach, contract := endedCashRoom(t, h)
	cc := consultationDraft(t, h, contractOwner, "/v1/coaching/contracts/"+contract+"/consultations", uuid.NewString(), "契約の相談")
	consultationAction(t, h, contractOwner, cc.ID, "submit", nil)
	if code := payConsultation(t, h, cc.ID, 1200); code != 200 {
		t.Fatal(code)
	}
	consultationAction(t, h, contractCoach, cc.ID, "accept", nil)
	consultationAction(t, h, contractCoach, cc.ID, "answer", map[string]any{"body": "契約の回答"})
	consultationAction(t, h, contractOwner, cc.ID, "clarify", map[string]any{"body": "確認です"})
	consultationAction(t, h, contractCoach, cc.ID, "clarification", map[string]any{"body": "補足です"})
	// サポートプランは回答済みでも、改めて購入できる（回数の上限は動画添削のあとの相談だけ）。
	consultationDraft(t, h, contractOwner, "/v1/coaching/contracts/"+contract+"/consultations", uuid.NewString(), "2回目の契約の相談")
}

// やりとりのゲート（届出待ち）が閉じていても、新しいゲートが開いていれば動画添削のあとの追加相談だけは送れる。
func TestReviewConsultationOpenWhileMessagingClosed(t *testing.T) {
	h, _, owner, coach, root := answeredCashReviewRoom(t, func(c *config.Config) {
		c.CoachingMessaging = false
		c.CoachingConsultation = true
	})
	c := consultationDraft(t, h, owner, "/v1/coaching/requests/"+root+"/consultations", uuid.NewString(), "届出前でも送れる質問")
	consultationAction(t, h, owner, c.ID, "submit", nil)
	if code := payConsultation(t, h, c.ID, 600); code != 200 {
		t.Fatal(code)
	}
	consultationAction(t, h, coach, c.ID, "accept", nil)
	consultationAction(t, h, coach, c.ID, "answer", map[string]any{"body": "届出前でも返せる回答"})

	contractOwner, _, contract := endedCashRoom(t, h)
	cc := consultationDraft(t, h, contractOwner, "/v1/coaching/contracts/"+contract+"/consultations", uuid.NewString(), "契約の相談")
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+cc.ID+"/submit", contractOwner, nil); r.Code != http.StatusConflict || !strings.Contains(r.Body.String(), "messaging_closed") {
		t.Fatalf("contract submit %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+cc.ID+"/checkout", contractOwner, nil); r.Code != http.StatusConflict || !strings.Contains(r.Body.String(), "messaging_closed") {
		t.Fatalf("contract checkout %d %s", r.Code, r.Body.String())
	}
}
