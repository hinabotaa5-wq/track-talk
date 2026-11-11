package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// #549: 県別大会の行を消しても、既存ユーザーの「目標の大会」の表示は壊れない。
// target_meets は名前と日付を写し取って持つので、元の行がなくても読める。
func TestTargetMeetSurvivesCompetitionDeletion(t *testing.T) {
	h := setup(t)
	pool := openPool(t)
	ctx := context.Background()
	compID := uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM competitions WHERE id=$1`, compID)
	})
	if _, err := pool.Exec(ctx, `INSERT INTO competitions (id,name,starts_on,ends_on,location,prefecture,season_year,organizer_url,source_url) VALUES ($1,'国際テスト大会','2027-09-01','2027-09-10','東京','00',2027,'https://example.jp/','https://example.jp/')`, compID); err != nil {
		t.Fatal(err)
	}
	me := token(t, uuid.New())
	put := func(ref string) *httptest.ResponseRecorder {
		return doJSON(t, h, http.MethodPut, "/v1/me/target-meet", me, map[string]string{"source": "competition", "ref": ref})
	}
	if rec := put(compID.String()); rec.Code != http.StatusOK {
		t.Fatalf("set %d %s", rec.Code, rec.Body.String())
	}
	if _, err := pool.Exec(ctx, `DELETE FROM competitions WHERE id=$1`, compID); err != nil {
		t.Fatal(err)
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/me/target-meet", me, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		TargetMeet *struct {
			Name string `json:"name"`
		} `json:"target_meet"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.TargetMeet == nil || body.TargetMeet.Name != "国際テスト大会" {
		t.Fatalf("snapshot %+v %s", body.TargetMeet, rec.Body.String())
	}
}
