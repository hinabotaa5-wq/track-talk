package httpx_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// コーチに見せる追加質問・追加相談は、送信（チケット）か支払い（都度払い）が一度でも成立したものだけ。
// 下書きや支払い待ちのまま取り下げた・期限が切れたものは、部屋の一覧にも個別の操作にも出さない。

func roomBody(t *testing.T, h http.Handler, tok, path string) string {
	t.Helper()
	r := doJSON(t, h, "GET", path, tok, nil)
	if r.Code != 200 {
		t.Fatalf("room %s %d %s", path, r.Code, r.Body.String())
	}
	return r.Body.String()
}

func assertHiddenFromCoach(t *testing.T, h http.Handler, coach, path, id, marker string) {
	t.Helper()
	if body := roomBody(t, h, coach, path); strings.Contains(body, marker) || strings.Contains(body, id) {
		t.Fatalf("coach sees unsent consultation: %s", body)
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+id+"/accept", coach, nil); r.Code != 404 {
		t.Fatalf("coach individual access %d %s", r.Code, r.Body.String())
	}
}

func marker() string { return "見えない印-" + uuid.NewString()[:8] }

// 1. 下書きを取り下げても、コーチの一覧に出ず、個別の操作は 404。依頼者本人には従来どおり見える。
func TestCoachDoesNotSeeCancelledDraft(t *testing.T) {
	h, _, owner, coach, root := answeredCashReviewRoom(t, nil)
	path := "/v1/coaching/requests/" + root + "/consultations"
	m := marker()
	c := consultationDraft(t, h, owner, path, uuid.NewString(), m)
	assertHiddenFromCoach(t, h, coach, path, c.ID, m)
	if got := consultationAction(t, h, owner, c.ID, "cancel", nil); got.Status != "cancelled" {
		t.Fatalf("status %s", got.Status)
	}
	assertHiddenFromCoach(t, h, coach, path, c.ID, m)
	if !strings.Contains(roomBody(t, h, owner, path), m) {
		t.Fatal("requester lost own cancelled draft")
	}
}

// 2. 支払い待ちのまま取り下げても、コーチには出ない。
func TestCoachDoesNotSeeCancelledUnpaid(t *testing.T) {
	h, _, owner, coach, root := answeredCashReviewRoom(t, nil)
	path := "/v1/coaching/requests/" + root + "/consultations"
	m := marker()
	c := consultationDraft(t, h, owner, path, uuid.NewString(), m)
	if got := consultationAction(t, h, owner, c.ID, "submit", nil); got.Status != "awaiting_payment" {
		t.Fatalf("status %s", got.Status)
	}
	assertHiddenFromCoach(t, h, coach, path, c.ID, m)
	consultationAction(t, h, owner, c.ID, "cancel", nil)
	assertHiddenFromCoach(t, h, coach, path, c.ID, m)
	if !strings.Contains(roomBody(t, h, owner, path), m) {
		t.Fatal("requester lost own cancelled consultation")
	}
}

// 3. 支払い待ちのまま期限が切れても、コーチには出ない。
func TestCoachDoesNotSeeExpiredUnpaid(t *testing.T) {
	h, _, owner, coach, root := answeredCashReviewRoom(t, nil)
	path := "/v1/coaching/requests/" + root + "/consultations"
	m := marker()
	c := consultationDraft(t, h, owner, path, uuid.NewString(), m)
	consultationAction(t, h, owner, c.ID, "submit", nil)
	execSQLArgs(t, `UPDATE coaching_consultations SET expires_at=now()-interval '1 minute' WHERE id=$1`, c.ID)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_consultations WHERE id=$1`, []any{c.ID}, &status)
	if status != "expired" {
		t.Fatal(status)
	}
	assertHiddenFromCoach(t, h, coach, path, c.ID, m)
	if !strings.Contains(roomBody(t, h, owner, path), m) {
		t.Fatal("requester lost own expired consultation")
	}
}

// 4. 支払いが成立したあと、コーチが受けないまま期限が切れたものは、これまでどおりコーチに見える。
func TestCoachStillSeesPaidThenExpired(t *testing.T) {
	h, _, owner, coach, root := answeredCashReviewRoom(t, nil)
	path := "/v1/coaching/requests/" + root + "/consultations"
	m := marker()
	c := consultationDraft(t, h, owner, path, uuid.NewString(), m)
	consultationAction(t, h, owner, c.ID, "submit", nil)
	if code := payConsultation(t, h, c.ID, 500); code != 200 {
		t.Fatalf("pay %d", code)
	}
	execSQLArgs(t, `UPDATE coaching_consultations SET respond_by=now()-interval '1 minute' WHERE id=$1`, c.ID)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_consultations WHERE id=$1`, []any{c.ID}, &status)
	if status != "expired" {
		t.Fatal(status)
	}
	if !strings.Contains(roomBody(t, h, coach, path), m) {
		t.Fatal("coach lost paid consultation")
	}
}

// 5. 支払い成立後の通常の流れ（受ける・回答する・確認に答える）は、コーチから見え続ける（継続プランの部屋）。
func TestCoachSeesPaidConsultationThroughAnswer(t *testing.T) {
	h, _ := setupStripe(t)
	owner, coach, root := endedCashRoom(t, h)
	path := "/v1/coaching/contracts/" + root + "/consultations"
	m := marker()
	c := consultationDraft(t, h, owner, path, uuid.NewString(), m)
	consultationAction(t, h, owner, c.ID, "submit", nil)
	if code := payConsultation(t, h, c.ID, 1200); code != 200 {
		t.Fatalf("pay %d", code)
	}
	if !strings.Contains(roomBody(t, h, coach, path), m) {
		t.Fatal("coach cannot see offered")
	}
	consultationAction(t, h, coach, c.ID, "accept", nil)
	consultationAction(t, h, coach, c.ID, "answer", map[string]any{"body": "回答です"})
	consultationAction(t, h, owner, c.ID, "clarify", map[string]any{"body": "確認です"})
	consultationAction(t, h, coach, c.ID, "clarification", map[string]any{"body": "補足です"})
	if !strings.Contains(roomBody(t, h, coach, path), m) {
		t.Fatal("coach cannot see answered")
	}
}

// 6. 保護者からは、取り下げたあとも従来どおり見える（保護者が払う18歳未満の継続プランの相談）。
func TestGuardianStillSeesCancelledUnpaid(t *testing.T) {
	h, _ := setupStripe(t)
	minor, guardian := guardianMinor(t, h)
	coach, root := minorAccepted(t, h, minor)
	execSQLArgs(t, `INSERT INTO coaching_contract_consents(contract_id,party,profile_id,version,report_review,contact_detection) VALUES ($1,'guardian',NULL,1,true,true)`, root)
	execSQLArgs(t, `UPDATE coaching_contracts SET status='ended',starts_at=now()-interval '8 days',ends_at=now()-interval '1 day',additional_consultation_price_yen=1200 WHERE id=$1`, root)
	path := "/v1/coaching/contracts/" + root + "/consultations"
	gpath := "/v1/guardian/" + guardian + "/contracts/" + root + "/consultations"
	m := marker()
	c := consultationDraft(t, h, minor, path, uuid.NewString(), m)
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/submit", minor, nil); r.Code != 200 {
		t.Fatalf("submit %d %s", r.Code, r.Body.String())
	}
	if !strings.Contains(roomBody(t, h, "", gpath), m) {
		t.Fatal("guardian cannot see awaiting payment")
	}
	consultationAction(t, h, minor, c.ID, "cancel", nil)
	if !strings.Contains(roomBody(t, h, "", gpath), m) {
		t.Fatal("guardian lost cancelled consultation")
	}
	if !strings.Contains(roomBody(t, h, minor, path), m) {
		t.Fatal("minor lost own cancelled consultation")
	}
	assertHiddenFromCoach(t, h, coach, path, c.ID, m)
}

// 7. 継続プランの部屋でも、取り下げた下書きはコーチに出ない。
func TestCoachDoesNotSeeCancelledDraftInContractRoom(t *testing.T) {
	h, _ := setupStripe(t)
	owner, coach, root := endedCashRoom(t, h)
	path := "/v1/coaching/contracts/" + root + "/consultations"
	m := marker()
	c := consultationDraft(t, h, owner, path, uuid.NewString(), m)
	consultationAction(t, h, owner, c.ID, "cancel", nil)
	assertHiddenFromCoach(t, h, coach, path, c.ID, m)
}

// 8. 動画つきの下書きでも、取り下げたあとのコーチの応答に動画は含まれない。
func TestCoachDoesNotSeeCancelledDraftVideo(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	owner, coach, root := endedCashRoom(t, h)
	path := "/v1/coaching/contracts/" + root + "/consultations"
	video := signedCoachingVideo(t, h, owner)
	m := marker()
	r := doJSON(t, h, "POST", path, owner, map[string]any{"request_key": uuid.NewString(), "body": m, "video_public_id": video})
	if r.Code != 200 {
		t.Fatalf("draft %d %s", r.Code, r.Body.String())
	}
	var id string
	queryRowArgs(t, `SELECT id::text FROM coaching_consultations WHERE video_public_id=$1`, []any{video}, &id)
	if !strings.Contains(roomBody(t, h, owner, path), video) {
		t.Fatal("requester cannot see own video")
	}
	consultationAction(t, h, owner, id, "cancel", nil)
	body := roomBody(t, h, coach, path)
	if strings.Contains(body, video) || strings.Contains(body, m) {
		t.Fatalf("coach sees cancelled video: %s", body)
	}
	assertHiddenFromCoach(t, h, coach, path, id, m)
}

// 9. 支払いの期限を過ぎてから決済が届き、そのまま返金（資金解放）で終わったものは、コーチに届いていないので出さない。
// 依頼者本人には従来どおり見える。
func TestCoachDoesNotSeeLatePaymentRefund(t *testing.T) {
	h, f, owner, coach, root := answeredCashReviewRoom(t, nil)
	path := "/v1/coaching/requests/" + root + "/consultations"
	m := marker()
	c := consultationDraft(t, h, owner, path, uuid.NewString(), m)
	consultationAction(t, h, owner, c.ID, "submit", nil)
	// 支払い待ちのまま支払い期限を過ぎたあとに、決済完了が届く。
	execSQLArgs(t, `UPDATE coaching_consultations SET expires_at=now()-interval '1 minute' WHERE id=$1`, c.ID)
	if code := payConsultation(t, h, c.ID, 500); code != 200 {
		t.Fatalf("late pay %d", code)
	}
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_consultations WHERE id=$1`, []any{c.ID}, &status)
	if status != "expired" {
		t.Fatalf("status %s", status)
	}
	// コーチには届かず、資金は解放（返金）側で処理される。
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var state string
	queryRowArgs(t, `SELECT payment_state FROM coaching_consultations WHERE id=$1`, []any{c.ID}, &state)
	if state != "refunded" || len(f.refundList()) != 1 {
		t.Fatalf("refund %s %+v", state, f.refundList())
	}
	assertHiddenFromCoach(t, h, coach, path, c.ID, m)
	if !strings.Contains(roomBody(t, h, owner, path), m) {
		t.Fatal("requester lost own late-paid consultation")
	}
}

// 10. 期限内に支払いが確定しても、確定時の検証で無効（契約停止など）となり offered から即 expired に閉じられた
// ものは、コーチに届いていないので出さない（判定基準の自然な帰結。修正前は「期限切れは respond_by があれば見せる」
// だったので見えていた）。
func TestCoachDoesNotSeePaidThenInvalidatedImmediately(t *testing.T) {
	h, f := setupStripe(t)
	owner, coach, root := endedCashRoom(t, h)
	path := "/v1/coaching/contracts/" + root + "/consultations"
	m := marker()
	c := consultationDraft(t, h, owner, path, uuid.NewString(), m)
	consultationAction(t, h, owner, c.ID, "submit", nil)
	// 支払い画面までは作れたが、決済完了の前に契約が停止された。
	execSQLArgs(t, `UPDATE coaching_contracts SET status='suspended',suspended_at=now() WHERE id=$1`, root)
	if code := payConsultation(t, h, c.ID, 1200); code != 200 {
		t.Fatalf("pay %d", code)
	}
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_consultations WHERE id=$1`, []any{c.ID}, &status)
	if status != "expired" {
		t.Fatalf("status %s", status)
	}
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var state string
	queryRowArgs(t, `SELECT payment_state FROM coaching_consultations WHERE id=$1`, []any{c.ID}, &state)
	if state != "refunded" || len(f.refundList()) != 1 {
		t.Fatalf("refund %s %+v", state, f.refundList())
	}
	assertHiddenFromCoach(t, h, coach, path, c.ID, m)
	if !strings.Contains(roomBody(t, h, owner, path), m) {
		t.Fatal("requester lost own invalidated consultation")
	}
}
