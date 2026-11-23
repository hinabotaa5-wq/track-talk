package httpx_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 1回プランの追加質問は販売を終了した（本体 #984）。添削（review）を対象にした新規作成だけを止め、
// 購入済みの追加質問の閲覧と、継続プランの追加相談（contract）は従来どおり動く。
func TestReviewConsultationSalesEnded(t *testing.T) {
	h, _, owner, coach, root := answeredCashReviewRoom(t, func(c *config.Config) { c.ReviewConsultationSales = false })
	path := "/v1/coaching/requests/" + root + "/consultations"

	// 部屋は読めるが、購入はできない。
	r := doJSON(t, h, "GET", path, owner, nil)
	var room struct {
		CanPurchase   bool              `json:"can_purchase"`
		BlockReason   string            `json:"block_reason"`
		Consultations []json.RawMessage `json:"consultations"`
	}
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &room) != nil || room.CanPurchase || !strings.Contains(room.BlockReason, "販売を終了") {
		t.Fatalf("room %d %s", r.Code, r.Body.String())
	}
	// 下書きの保存（新規作成の入口）も API で断る。
	if r := doJSON(t, h, "POST", path, owner, map[string]any{"request_key": uuid.NewString(), "body": "質問"}); r.Code == 200 || !strings.Contains(r.Body.String(), "consultation_sales_ended") {
		t.Fatalf("draft %d %s", r.Code, r.Body.String())
	}
	var count int
	queryRowArgs(t, `SELECT count(*) FROM coaching_consultations WHERE review_request_id=$1`, []any{root}, &count)
	if count != 0 {
		t.Fatalf("consultations created: %d", count)
	}

	// 販売終了の前に購入・回答済みだった追加質問は、選手・コーチとも従来どおり読める。
	id := uuid.New()
	execSQLArgs(t, `INSERT INTO coaching_consultations (id, requester_id, coach_id, request_key, review_request_id, minor, funding, price_yen, share_percent,
			status, question, answer, respond_by, paid_at, answered_at, offered_at, payment_state, payment_intent_id)
		SELECT $1::uuid, r.requester_id, r.coach_id, $1::uuid, r.id, false, 'cash', 500, 70, 'answered', '購入済みの質問', '購入済みの回答', now(), now(), now(), now(), 'paid', 'pi_sold_before_end_' || $1::text
		FROM review_requests r WHERE r.id=$2::uuid`, id, root)
	for _, tok := range []string{owner, coach} {
		r := doJSON(t, h, "GET", path, tok, nil)
		if r.Code != 200 || !strings.Contains(r.Body.String(), id.String()) || !strings.Contains(r.Body.String(), "購入済みの回答") {
			t.Fatalf("existing consultation %d %s", r.Code, r.Body.String())
		}
	}

	// 継続プランの追加相談は、作成・支払い・受諾・回答とも従来どおり。
	contractOwner, contractCoach, contract := endedCashRoom(t, h)
	cc := consultationDraft(t, h, contractOwner, "/v1/coaching/contracts/"+contract+"/consultations", uuid.NewString(), "契約の相談")
	consultationAction(t, h, contractOwner, cc.ID, "submit", nil)
	if code := payConsultation(t, h, cc.ID, 1200); code != 200 {
		t.Fatal(code)
	}
	consultationAction(t, h, contractCoach, cc.ID, "accept", nil)
	consultationAction(t, h, contractCoach, cc.ID, "answer", map[string]any{"body": "契約の回答"})
}
