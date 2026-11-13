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
		if detail.Code != http.StatusOK || !bytes.Equal(bytes.TrimSpace(detail.Body.Bytes()), entry) {
			t.Fatalf("detail %d %s, want %s", detail.Code, detail.Body.String(), entry)
		}
		t.Logf("JSON response bytes: two coaches %d, single coach %d (support consultation: %t)", list.Body.Len(), detail.Body.Len(), row.ID == first)
	}

	execSQLArgs(t, `UPDATE coach_profiles SET status='suspended', on_duty=false WHERE profile_id=$1`, first)
	hidden := doJSON(t, h, http.MethodGet, "/v1/coaches/"+first.String(), "", nil)
	missing := doJSON(t, h, http.MethodGet, "/v1/coaches/"+uuid.NewString(), "", nil)
	if hidden.Code != http.StatusNotFound || missing.Code != hidden.Code || missing.Body.String() != hidden.Body.String() {
		t.Fatalf("unpublished %d %s, missing %d %s", hidden.Code, hidden.Body.String(), missing.Code, missing.Body.String())
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
