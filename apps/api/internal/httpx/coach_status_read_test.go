package httpx_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// 追加質問の部屋と事前相談の個票も、ほかのコーチ向けの画面と同じく「いまも承認済みのコーチか」を確かめる。
// 止められたコーチは担当していても読めず（403）、戻されるとまた読める。依頼者・選手・保護者の見え方は変わらない。

func suspendCoachByAdmin(t *testing.T, h http.Handler, coach uuid.UUID) {
	t.Helper()
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(coach, "suspend"), adminToken(t, h), nil); rec.Code != http.StatusOK {
		t.Fatalf("suspend %d %s", rec.Code, rec.Body.String())
	}
}

func resumeCoachByAdmin(t *testing.T, h http.Handler, coach uuid.UUID) {
	t.Helper()
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(coach, "resume"), adminToken(t, h), nil); rec.Code != http.StatusOK {
		t.Fatalf("resume %d %s", rec.Code, rec.Body.String())
	}
}

func roomCoachID(t *testing.T, table, root string) uuid.UUID {
	t.Helper()
	var id string
	queryRowArgs(t, `SELECT coach_id::text FROM `+table+` WHERE id=$1`, []any{root}, &id)
	return uuid.MustParse(id)
}

// paidVideoConsultation は、動画つきの追加質問を送って支払いまで済ませ、コーチに届いた状態にする。
func paidVideoConsultation(t *testing.T, h http.Handler, owner, path string, amount int) (id, body, video string) {
	t.Helper()
	video = signedCoachingVideo(t, h, owner)
	body = marker()
	if r := doJSON(t, h, http.MethodPost, path, owner, map[string]any{"request_key": uuid.NewString(), "body": body, "video_public_id": video}); r.Code != http.StatusOK {
		t.Fatalf("draft %d %s", r.Code, r.Body.String())
	}
	queryRowArgs(t, `SELECT id::text FROM coaching_consultations WHERE video_public_id=$1`, []any{video}, &id)
	consultationAction(t, h, owner, id, "submit", nil)
	if code := payConsultation(t, h, id, amount); code != http.StatusOK {
		t.Fatalf("pay %d", code)
	}
	return id, body, video
}

func assertRoomForbidden(t *testing.T, h http.Handler, tok, path string, hidden ...string) {
	t.Helper()
	r := doJSON(t, h, http.MethodGet, path, tok, nil)
	if r.Code != http.StatusForbidden {
		t.Fatalf("suspended coach room %d, want 403: %s", r.Code, r.Body.String())
	}
	for _, s := range hidden {
		if strings.Contains(r.Body.String(), s) {
			t.Fatalf("suspended coach response contains %q: %s", s, r.Body.String())
		}
	}
}

// 1・3・4・7. 添削依頼の部屋：承認済みなら 200 → 停止で 403（質問文・動画を含まない）→ 戻すと 200。
// 依頼者本人は停止中も読め、当事者でない人は従来どおり 404。
func TestSuspendedCoachCannotReadReviewConsultationRoom(t *testing.T) {
	h, _, owner, coach, root := answeredCashReviewRoom(t, nil)
	path := "/v1/coaching/requests/" + root + "/consultations"
	_, body, video := paidVideoConsultation(t, h, owner, path, 500)
	if got := roomBody(t, h, coach, path); !strings.Contains(got, body) || !strings.Contains(got, video) {
		t.Fatalf("approved coach cannot see consultation: %s", got)
	}
	coachID := roomCoachID(t, "review_requests", root)

	suspendCoachByAdmin(t, h, coachID)
	assertRoomForbidden(t, h, coach, path, body, video)
	if got := roomBody(t, h, owner, path); !strings.Contains(got, body) || !strings.Contains(got, video) {
		t.Fatalf("requester lost own consultation: %s", got)
	}
	_, stranger := demoUser(t, h, bornYearsAgo(20))
	if r := doJSON(t, h, http.MethodGet, path, stranger, nil); r.Code != http.StatusNotFound {
		t.Fatalf("stranger room %d %s", r.Code, r.Body.String())
	}

	resumeCoachByAdmin(t, h, coachID)
	if got := roomBody(t, h, coach, path); !strings.Contains(got, body) || !strings.Contains(got, video) {
		t.Fatalf("resumed coach cannot see consultation: %s", got)
	}
}

// 2・3・7. 継続プランの部屋でも同じ：承認済み 200 → 停止で 403 → 戻すと 200。依頼者は読め、第三者は 404。
func TestSuspendedCoachCannotReadContractConsultationRoom(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	owner, coach, root := endedCashRoom(t, h)
	path := "/v1/coaching/contracts/" + root + "/consultations"
	_, body, video := paidVideoConsultation(t, h, owner, path, 1200)
	if got := roomBody(t, h, coach, path); !strings.Contains(got, body) || !strings.Contains(got, video) {
		t.Fatalf("approved coach cannot see consultation: %s", got)
	}
	coachID := roomCoachID(t, "coaching_contracts", root)

	suspendCoachByAdmin(t, h, coachID)
	assertRoomForbidden(t, h, coach, path, body, video)
	if got := roomBody(t, h, owner, path); !strings.Contains(got, body) {
		t.Fatalf("requester lost own consultation: %s", got)
	}
	_, stranger := demoUser(t, h, bornYearsAgo(20))
	if r := doJSON(t, h, http.MethodGet, path, stranger, nil); r.Code != http.StatusNotFound {
		t.Fatalf("stranger room %d %s", r.Code, r.Body.String())
	}

	resumeCoachByAdmin(t, h, coachID)
	if got := roomBody(t, h, coach, path); !strings.Contains(got, body) {
		t.Fatalf("resumed coach cannot see consultation: %s", got)
	}
}

// 3. 保護者のリンクからと18歳未満の本人からは、担当コーチが止められても従来どおり部屋が読める。
func TestSuspendedCoachRoomStillVisibleToMinorAndGuardian(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	minor, guardian := guardianMinor(t, h)
	coach, root := minorAccepted(t, h, minor)
	execSQLArgs(t, `INSERT INTO coaching_contract_consents(contract_id,party,profile_id,version,report_review,contact_detection) VALUES ($1,'guardian',NULL,1,true,true)`, root)
	execSQLArgs(t, `UPDATE coaching_contracts SET status='ended',starts_at=now()-interval '8 days',ends_at=now()-interval '1 day',additional_consultation_price_yen=1200 WHERE id=$1`, root)
	path := "/v1/coaching/contracts/" + root + "/consultations"
	gpath := "/v1/guardian/" + guardian + "/contracts/" + root + "/consultations"
	m := marker()
	c := consultationDraft(t, h, minor, path, uuid.NewString(), m)
	if r := doJSON(t, h, http.MethodPost, "/v1/coaching/consultations/"+c.ID+"/submit", minor, nil); r.Code != http.StatusOK {
		t.Fatalf("submit %d %s", r.Code, r.Body.String())
	}
	if !strings.Contains(roomBody(t, h, "", gpath), m) {
		t.Fatal("guardian cannot see consultation before coach suspension")
	}

	suspendCoachByAdmin(t, h, roomCoachID(t, "coaching_contracts", root))
	if !strings.Contains(roomBody(t, h, "", gpath), m) {
		t.Fatal("guardian lost consultation after coach suspension")
	}
	if !strings.Contains(roomBody(t, h, minor, path), m) {
		t.Fatal("minor lost consultation after coach suspension")
	}
	assertRoomForbidden(t, h, coach, path, m)
}

// 8. 書き込み系（受ける・断る・回答する）は、止められたコーチには従来どおり通らない（応答コードは変えない）。
func TestSuspendedCoachConsultationWritesUnchanged(t *testing.T) {
	h, _, owner, coach, root := answeredCashReviewRoom(t, nil)
	path := "/v1/coaching/requests/" + root + "/consultations"
	offered, _, _ := paidVideoConsultation(t, h, owner, path, 500)
	coachID := roomCoachID(t, "review_requests", root)

	suspendCoachByAdmin(t, h, coachID)
	for _, action := range []string{"accept", "decline"} {
		if r := doJSON(t, h, http.MethodPost, "/v1/coaching/consultations/"+offered+"/"+action, coach, nil); r.Code != http.StatusUnprocessableEntity {
			t.Errorf("suspended coach %s %d, want 422: %s", action, r.Code, r.Body.String())
		}
	}
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_consultations WHERE id=$1`, []any{offered}, &status)
	if status != "offered" {
		t.Fatalf("status changed to %s", status)
	}

	resumeCoachByAdmin(t, h, coachID)
	consultationAction(t, h, coach, offered, "accept", nil)
	suspendCoachByAdmin(t, h, coachID)
	if r := doJSON(t, h, http.MethodPost, "/v1/coaching/consultations/"+offered+"/answer", coach, map[string]any{"body": "停止中の回答"}); r.Code != http.StatusUnprocessableEntity {
		t.Errorf("suspended coach answer %d, want 422: %s", r.Code, r.Body.String())
	}
	queryRowArgs(t, `SELECT status FROM coaching_consultations WHERE id=$1`, []any{offered}, &status)
	if status != "in_review" {
		t.Fatalf("status changed to %s", status)
	}
}

// 5・6・7. 事前相談の個票：コーチは承認済みなら 200 → 停止で 403（一覧も 403）→ 戻すと 200。
// 選手本人は相手コーチが止められても読め、当事者でない人は従来どおり 404。
func TestSuspendedCoachCannotReadSupportConsult(t *testing.T) {
	h := setup(t)
	coachID, tc := consultCoach(t, h)
	_, ta := demoUser(t, h, bornYearsAgo(20))
	id := sendConsult(t, h, ta, coachID, consultAnswersBody())
	path := "/v1/support-consults/" + id
	if r := doJSON(t, h, http.MethodGet, path, tc, nil); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"role":"coach"`) {
		t.Fatalf("approved coach consult %d %s", r.Code, r.Body.String())
	}

	suspendCoachByAdmin(t, h, coachID)
	r := doJSON(t, h, http.MethodGet, path, tc, nil)
	if r.Code != http.StatusForbidden {
		t.Fatalf("suspended coach consult %d, want 403: %s", r.Code, r.Body.String())
	}
	for _, s := range []string{"スタートで出遅れます", "週に何回くらい動画を送ればよいですか", id} {
		if strings.Contains(r.Body.String(), s) {
			t.Fatalf("suspended coach response contains %q: %s", s, r.Body.String())
		}
	}
	if r := doJSON(t, h, http.MethodGet, "/v1/coach/me/support-consults", tc, nil); r.Code != http.StatusForbidden {
		t.Fatalf("suspended coach consult list %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, http.MethodGet, path, ta, nil); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"role":"athlete"`) || !strings.Contains(r.Body.String(), "スタートで出遅れます") {
		t.Fatalf("athlete consult after coach suspension %d %s", r.Code, r.Body.String())
	}
	_, stranger := demoUser(t, h, bornYearsAgo(20))
	if r := doJSON(t, h, http.MethodGet, path, stranger, nil); r.Code != http.StatusNotFound {
		t.Fatalf("stranger consult %d %s", r.Code, r.Body.String())
	}

	resumeCoachByAdmin(t, h, coachID)
	if r := doJSON(t, h, http.MethodGet, path, tc, nil); r.Code != http.StatusOK {
		t.Fatalf("resumed coach consult %d %s", r.Code, r.Body.String())
	}
}
