package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

type identityStatusView struct {
	Registered *bool `json:"registered"`
	Applies    bool  `json:"applies"`
}

func decodeIdentityStatus(t *testing.T, body []byte) identityStatusView {
	t.Helper()
	var out identityStatusView
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		t.Fatalf("decode identity-status: %v %s", err, string(body))
	}
	return out
}

func identityStub(t *testing.T, status int) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/internal/registration/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(s.Close)
	return s
}

func approvedCoachWithServices(t *testing.T, h http.Handler, services []string) (uuid.UUID, string) {
	t.Helper()
	coach := uuid.New()
	tc := token(t, coach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tc, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	withCoachPhoto(t, coach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+coach.String()+"/approve", adminToken(t, h), nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d %s", rec.Code, rec.Body.String())
	}
	if services != nil {
		if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/services", tc, map[string]any{"service_types": services}); rec.Code != http.StatusOK {
			t.Fatalf("services %d %s", rec.Code, rec.Body.String())
		}
	}
	return coach, tc
}

func TestCoachIdentityStatus(t *testing.T) {
	t.Run("registered true/false", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			remote     int
			registered bool
		}{
			{"registered", http.StatusNoContent, true},
			{"unregistered", http.StatusNotFound, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				stub := identityStub(t, tc.remote)
				h := setupConfig(t, func(cfg *config.Config) {
					cfg.IdentityReaderURL = stub.URL
					cfg.IdentityReaderToken = "synthetic-read-only-identity-token"
				})
				_, tok := approvedCoachWithServices(t, h, []string{"video_review"})
				rec := doJSON(t, h, http.MethodGet, "/v1/coach/me/identity-status", tok, nil)
				if rec.Code != http.StatusOK {
					t.Fatalf("status %d %s", rec.Code, rec.Body.String())
				}
				out := decodeIdentityStatus(t, rec.Body.Bytes())
				if out.Registered == nil || *out.Registered != tc.registered {
					t.Fatalf("registered = %v, want %v (%s)", out.Registered, tc.registered, rec.Body.String())
				}
				if out.Applies {
					t.Fatalf("applies should be false for video_review only (%s)", rec.Body.String())
				}
			})
		}
	})

	t.Run("applies via services and via support plan", func(t *testing.T) {
		stub := identityStub(t, http.StatusNoContent)
		h := setupConfig(t, func(cfg *config.Config) {
			cfg.IdentityReaderURL = stub.URL
			cfg.IdentityReaderToken = "synthetic-read-only-identity-token"
		})

		_, inPerson := approvedCoachWithServices(t, h, []string{"in_person"})
		if rec := doJSON(t, h, http.MethodGet, "/v1/coach/me/identity-status", inPerson, nil); rec.Code != http.StatusOK {
			t.Fatalf("in_person %d %s", rec.Code, rec.Body.String())
		} else if out := decodeIdentityStatus(t, rec.Body.Bytes()); !out.Applies {
			t.Fatalf("in_person applies should be true (%s)", rec.Body.String())
		}

		_, competition := approvedCoachWithServices(t, h, []string{"competition_support"})
		if rec := doJSON(t, h, http.MethodGet, "/v1/coach/me/identity-status", competition, nil); rec.Code != http.StatusOK {
			t.Fatalf("competition_support %d %s", rec.Code, rec.Body.String())
		} else if out := decodeIdentityStatus(t, rec.Body.Bytes()); !out.Applies {
			t.Fatalf("competition_support applies should be true (%s)", rec.Body.String())
		}

		// サポートプランを持っていれば、担当サービスが空でも対象になる。
		coach, tok := approvedCoachWithServices(t, h, []string{"video_review"})
		if rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/pledge", tok, map[string]any{"agree": true}); rec.Code != http.StatusOK {
			t.Fatalf("pledge %d %s", rec.Code, rec.Body.String())
		}
		body := planBody()
		if rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tok, body); rec.Code != http.StatusCreated {
			t.Fatalf("create plan %d %s", rec.Code, rec.Body.String())
		}
		execSQLArgs(t, `UPDATE coaching_plans SET service_type='competition_support' WHERE coach_id=$1`, coach)
		execSQLArgs(t, `UPDATE coach_profiles SET service_types='{}' WHERE profile_id=$1`, coach)
		rec := doJSON(t, h, http.MethodGet, "/v1/coach/me/identity-status", tok, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("plan %d %s", rec.Code, rec.Body.String())
		}
		if out := decodeIdentityStatus(t, rec.Body.Bytes()); !out.Applies {
			t.Fatalf("support plan applies should be true (%s)", rec.Body.String())
		}
	})

	t.Run("unknown when unconfigured or service error", func(t *testing.T) {
		h := setupConfig(t, func(cfg *config.Config) {
			cfg.IdentityReaderURL = ""
			cfg.IdentityReaderToken = ""
		})
		_, tok := approvedCoachWithServices(t, h, []string{"in_person"})
		rec := doJSON(t, h, http.MethodGet, "/v1/coach/me/identity-status", tok, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("unconfigured %d %s", rec.Code, rec.Body.String())
		}
		out := decodeIdentityStatus(t, rec.Body.Bytes())
		if out.Registered != nil {
			t.Fatalf("unconfigured registered should be null (%s)", rec.Body.String())
		}
		if !out.Applies {
			t.Fatalf("unconfigured applies should still be true (%s)", rec.Body.String())
		}

		stub := identityStub(t, http.StatusInternalServerError)
		h = setupConfig(t, func(cfg *config.Config) {
			cfg.IdentityReaderURL = stub.URL
			cfg.IdentityReaderToken = "synthetic-read-only-identity-token"
		})
		_, tok = approvedCoachWithServices(t, h, []string{"in_person"})
		rec = doJSON(t, h, http.MethodGet, "/v1/coach/me/identity-status", tok, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("service error %d %s", rec.Code, rec.Body.String())
		}
		if out := decodeIdentityStatus(t, rec.Body.Bytes()); out.Registered != nil {
			t.Fatalf("service error registered should be null (%s)", rec.Body.String())
		}
	})

	t.Run("not approved or signed out cannot read", func(t *testing.T) {
		stub := identityStub(t, http.StatusNoContent)
		h := setupConfig(t, func(cfg *config.Config) {
			cfg.IdentityReaderURL = stub.URL
			cfg.IdentityReaderToken = "synthetic-read-only-identity-token"
		})

		if rec := doJSON(t, h, http.MethodGet, "/v1/coach/me/identity-status", "", nil); rec.Code != http.StatusUnauthorized {
			t.Fatalf("signed out %d %s", rec.Code, rec.Body.String())
		}

		// 応募なし。
		fresh := token(t, uuid.New())
		if rec := doJSON(t, h, http.MethodGet, "/v1/coach/me/identity-status", fresh, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("no application %d %s", rec.Code, rec.Body.String())
		}

		// 審査待ち。
		pending := uuid.New()
		tp := token(t, pending)
		if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tp, coachApplicationBody()); rec.Code != http.StatusOK {
			t.Fatalf("apply %d", rec.Code)
		}
		if rec := doJSON(t, h, http.MethodGet, "/v1/coach/me/identity-status", tp, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("pending %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("returns only state", func(t *testing.T) {
		stub := identityStub(t, http.StatusNoContent)
		h := setupConfig(t, func(cfg *config.Config) {
			cfg.IdentityReaderURL = stub.URL
			cfg.IdentityReaderToken = "synthetic-read-only-identity-token"
		})
		_, tok := approvedCoachWithServices(t, h, []string{"in_person"})
		rec := doJSON(t, h, http.MethodGet, "/v1/coach/me/identity-status", tok, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		for _, leak := range []string{"synthetic-read-only-identity-token", "氏名", "住所", "電話", "版", "時刻", "version", "phone", "address", "legal_name"} {
			if strings.Contains(body, leak) {
				t.Fatalf("response leaks %q: %s", leak, body)
			}
		}
	})
}
