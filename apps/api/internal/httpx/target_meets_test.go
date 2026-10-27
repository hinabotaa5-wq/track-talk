package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// 目標の大会は1人1大会。別の大会をセットすると置き換わり、名前と日付は大会一覧から写す。
func TestTargetMeetSetReplaceAndClear(t *testing.T) {
	h := setup(t)
	pool := openPool(t)
	ctx := context.Background()
	compID := uuid.New()
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM domestic_majors WHERE slug='target-check'`)
		pool.Exec(ctx, `DELETE FROM competitions WHERE id=$1`, compID)
	})
	if _, err := pool.Exec(ctx, `INSERT INTO domestic_majors (slug,year,name,category,venue,starts_on,ends_on,schedule_label,site_url) VALUES ('target-check',2026,'目標確認大会','open','競技場','2026-11-20','2026-11-22','2026年','')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO competitions (id,name,starts_on,ends_on,location,prefecture,season_year,organizer_url,source_url) VALUES ($1,'県記録会','2026-12-01',NULL,'','13',2026,'https://example.jp/','https://example.jp/')`, compID); err != nil {
		t.Fatal(err)
	}
	me, other := token(t, uuid.New()), token(t, uuid.New())
	type target struct {
		Source, Ref, Name, StartsOn string
		EndsOn                      *string `json:"ends_on"`
	}
	read := func(tok string) *target {
		t.Helper()
		rec := doJSON(t, h, http.MethodGet, "/v1/me/target-meet", tok, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("get %d %s", rec.Code, rec.Body.String())
		}
		var body struct {
			TargetMeet *struct {
				Source   string  `json:"source"`
				Ref      string  `json:"ref"`
				Name     string  `json:"name"`
				StartsOn string  `json:"starts_on"`
				EndsOn   *string `json:"ends_on"`
			} `json:"target_meet"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body.TargetMeet == nil {
			return nil
		}
		v := body.TargetMeet
		return &target{v.Source, v.Ref, v.Name, v.StartsOn, v.EndsOn}
	}

	if rec := doJSON(t, h, http.MethodGet, "/v1/me/target-meet", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous %d", rec.Code)
	}
	if got := read(me); got != nil {
		t.Fatalf("initial %+v", got)
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/target-meet", me, map[string]string{"source": "major", "ref": "target-check:2026"}); rec.Code != http.StatusOK {
		t.Fatalf("set major %d %s", rec.Code, rec.Body.String())
	}
	if got := read(me); got == nil || got.Name != "目標確認大会" || got.StartsOn != "2026-11-20" || got.EndsOn == nil || *got.EndsOn != "2026-11-22" {
		t.Fatalf("major %+v", got)
	}
	if got := read(other); got != nil {
		t.Fatalf("other user sees %+v", got)
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/target-meet", me, map[string]string{"source": "competition", "ref": compID.String()}); rec.Code != http.StatusOK {
		t.Fatalf("replace %d %s", rec.Code, rec.Body.String())
	}
	if got := read(me); got == nil || got.Name != "県記録会" || got.Ref != compID.String() || got.EndsOn != nil {
		t.Fatalf("replaced %+v", got)
	}
	for _, body := range []map[string]string{
		{"source": "major", "ref": "missing:2026"},
		{"source": "competition", "ref": uuid.NewString()},
		{"source": "competition", "ref": "not-a-uuid"},
	} {
		if rec := doJSON(t, h, http.MethodPut, "/v1/me/target-meet", me, body); rec.Code != http.StatusNotFound {
			t.Fatalf("%v: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/target-meet", me, map[string]string{"source": "world", "ref": "x"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad source %d", rec.Code)
	}
	if got := read(me); got == nil || got.Name != "県記録会" {
		t.Fatalf("kept after failures %+v", got)
	}
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me/target-meet", me, nil); rec.Code != http.StatusOK {
		t.Fatalf("delete %d", rec.Code)
	}
	if got := read(me); got != nil {
		t.Fatalf("after delete %+v", got)
	}
}
