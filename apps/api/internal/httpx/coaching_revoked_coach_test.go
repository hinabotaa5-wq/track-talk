package httpx_test

import (
	"net/http"
	"testing"
)

// 承認を取り消された(停止した)コーチは、以前の申し込みでも、利用者とのやりとりを読んだり送ったりできない。
// 添削の依頼(TestReviewRevokedCoachAuthorization)と同じく、所有者であることと、いまも承認済みであることは別の条件。
func TestCoachingContractRevokedCoachAuthorization(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	path := "/v1/coaching/contracts/" + id
	if rec := doJSON(t, h, http.MethodPost, path+"/messages", tc, map[string]any{"body": "停止の前の連絡"}); rec.Code != http.StatusCreated {
		t.Fatalf("approved coach send %d %s", rec.Code, rec.Body.String())
	}
	before := len(listMessages(t, h, tu, id))
	execSQLArgs(t, `UPDATE coach_profiles SET status = 'suspended', on_duty = false
		WHERE profile_id = (SELECT coach_id FROM coaching_contracts WHERE id = $1)`, id)

	for _, op := range []struct {
		name, method, path string
		body               any
	}{
		{"contract", http.MethodGet, path, nil},
		{"messages", http.MethodGet, path + "/messages", nil},
		{"send", http.MethodPost, path + "/messages", map[string]any{"body": "停止したあとの連絡"}},
		{"support", http.MethodGet, path + "/support", nil},
	} {
		if rec := doJSON(t, h, op.method, op.path, tc, op.body); rec.Code != http.StatusForbidden {
			t.Errorf("%s: suspended coach got %d, want 403: %s", op.name, rec.Code, rec.Body.String())
		}
	}
	if after := len(listMessages(t, h, tu, id)); after != before {
		t.Errorf("suspended coach sent a message: %d -> %d", before, after)
	}
	// 利用者は、これまでどおり自分の申し込みを読める。
	if rec := doJSON(t, h, http.MethodGet, path, tu, nil); rec.Code != http.StatusOK {
		t.Errorf("requester read %d", rec.Code)
	}
}
