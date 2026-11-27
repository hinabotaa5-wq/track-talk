package httpx_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

func TestPublicCoachDetailMatchesList(t *testing.T) {
	h := setup(t)
	first, _ := consultCoach(t, h)
	second, _, _ := planCoach(t, h, 3)
	list := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil)
	var out struct {
		Coaches []json.RawMessage `json:"coaches"`
	}
	if list.Code != http.StatusOK || json.Unmarshal(list.Body.Bytes(), &out) != nil || len(out.Coaches) != 2 {
		t.Fatalf("coaches %d %s", list.Code, list.Body.String())
	}
	for _, entry := range out.Coaches {
		var row struct {
			ID             uuid.UUID         `json:"id"`
			Plans          []json.RawMessage `json:"plans"`
			SupportConsult json.RawMessage   `json:"support_consult"`
		}
		if err := json.Unmarshal(entry, &row); err != nil {
			t.Fatal(err)
		}
		if (row.ID != first && row.ID != second) || len(row.Plans) != 1 {
			t.Fatalf("unexpected coach or plans: %s", entry)
		}
		if row.ID == first && (len(row.SupportConsult) == 0 || bytes.Equal(row.SupportConsult, []byte("null"))) {
			t.Fatalf("support consultation missing: %s", entry)
		}
		detail := doJSON(t, h, http.MethodGet, "/v1/coaches/"+row.ID.String(), "", nil)
		// 「得られるもの」の追加項目と見本動画（本体 #1086）は詳細だけに出す。それ以外は一覧の1行と同じ。
		if detail.Code != http.StatusOK || !bytes.Equal(withoutDetailOnlyKeys(t, detail.Body.Bytes()), withoutDetailOnlyKeys(t, entry)) {
			t.Fatalf("detail %d %s, want %s", detail.Code, detail.Body.String(), entry)
		}
		if !bytes.Contains(detail.Body.Bytes(), []byte(`"offer_items":[]`)) || !bytes.Contains(detail.Body.Bytes(), []byte(`"demo_videos":[]`)) {
			t.Fatalf("detail-only keys missing: %s", detail.Body.String())
		}
		if bytes.Contains(entry, []byte(`"offer_items"`)) || bytes.Contains(entry, []byte(`"demo_videos"`)) {
			t.Fatalf("list must not carry detail-only keys: %s", entry)
		}
		assertPublicCoachShape(t, entry)
		assertPublicCoachShape(t, detail.Body.Bytes())
		t.Logf("JSON response bytes: two coaches %d, single coach %d (support consultation: %t)", list.Body.Len(), detail.Body.Len(), row.ID == first)
	}

	execSQLArgs(t, `UPDATE coach_profiles SET status='suspended', on_duty=false WHERE profile_id=$1`, first)
	hidden := doJSON(t, h, http.MethodGet, "/v1/coaches/"+first.String(), "", nil)
	missing := doJSON(t, h, http.MethodGet, "/v1/coaches/"+uuid.NewString(), "", nil)
	if hidden.Code != http.StatusNotFound || missing.Code != hidden.Code || missing.Body.String() != hidden.Body.String() {
		t.Fatalf("unpublished %d %s, missing %d %s", hidden.Code, hidden.Body.String(), missing.Code, missing.Body.String())
	}
}

// withoutDetailOnlyKeys は、詳細だけに出すキー（本体 #1086）を除いた JSON（キー順は encoding/json の並び）。
func withoutDetailOnlyKeys(t *testing.T, raw []byte) []byte {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	delete(m, "offer_items")
	delete(m, "demo_videos")
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// 公開コーチの応答に評価の件数・平均は出さない（本体 #821）。それ以外のキーは従来どおり返す。
func assertPublicCoachShape(t *testing.T, raw []byte) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"rating_count", "rating_average"} {
		if _, ok := m[k]; ok {
			t.Fatalf("%s must not be exposed: %s", k, raw)
		}
	}
	for _, k := range []string{"id", "nickname", "photo_url", "tier", "events", "service_types", "best_record", "achievements", "years", "sns_urls", "hitokoto", "plans", "support_consult"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("%s missing: %s", k, raw)
		}
	}
}

func TestPublicCoachDetailUsesListGate(t *testing.T) {
	h := setupConfig(t, func(cfg *config.Config) { cfg.CoachingCoachList = false })
	list := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil)
	detail := doJSON(t, h, http.MethodGet, "/v1/coaches/"+uuid.NewString(), "", nil)
	if list.Code != http.StatusNotFound || detail.Code != list.Code || detail.Body.String() != list.Body.String() {
		t.Fatalf("list %d %s, detail %d %s", list.Code, list.Body.String(), detail.Code, detail.Body.String())
	}
}
