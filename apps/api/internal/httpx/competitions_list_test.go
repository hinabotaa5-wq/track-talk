package httpx_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// #549: 県別（01〜47）の取り込み・表示は廃止。GET /v1/competitions は国際大会（00）だけを返す。
func TestListCompetitionsOnlyInternational(t *testing.T) {
	h := setup(t)
	if rec := doJSON(t, h, http.MethodGet, "/v1/competitions?prefecture=00", "", nil); rec.Code != http.StatusOK {
		t.Fatalf("international %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/competitions?prefecture=13", "", nil); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("prefectural %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/competitions", "", nil); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing %d %s", rec.Code, rec.Body.String())
	}
}

// #704: 「目標の大会をセットする」機能は廃止した。旧パスは 404 のままにする。
func TestTargetMeetRoutesRemoved(t *testing.T) {
	h := setup(t)
	me := token(t, uuid.New())
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		if rec := doJSON(t, h, method, "/v1/me/target-meet", me, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("%s: got %d, want 404 (%s)", method, rec.Code, rec.Body.String())
		}
	}
}
