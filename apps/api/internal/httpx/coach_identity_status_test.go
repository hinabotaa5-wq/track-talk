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

	// applies は受付設定の accepting で決まり、担当サービスだけでは対象にならない。
	// 受付設定の行は保存ゲートを通さずテスト用DBへ直接作る（到達状態は同じ）。
	t.Run("applies follows accepting, not services", func(t *testing.T) {
		stub := identityStub(t, http.StatusNotFound)
		h := setupConfig(t, func(cfg *config.Config) {
			cfg.IdentityReaderURL = stub.URL
			cfg.IdentityReaderToken = "synthetic-read-only-identity-token"
		})
		getApplies := func(t *testing.T, tok string) identityStatusView {
			t.Helper()
			rec := doJSON(t, h, http.MethodGet, "/v1/coach/me/identity-status", tok, nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d %s", rec.Code, rec.Body.String())
			}
			return decodeIdentityStatus(t, rec.Body.Bytes())
		}

		// 細目1：受付中ならプランなしでも対象（金庫未登録なので registered=false）。
		coach, accepting := approvedCoachWithServices(t, h, []string{"video_review"})
		execSQLArgs(t, `INSERT INTO support_consult_settings (coach_id, accepting) VALUES ($1, true)`, coach)
		if out := getApplies(t, accepting); !out.Applies {
			t.Fatalf("accepting applies should be true")
		} else if out.Registered == nil || *out.Registered {
			t.Fatalf("accepting registered should be false, got %v", out.Registered)
		}

		// 細目2：受付停止中は担当サービスだけ先行しても対象外（発端ケース）。
		stoppedCoach, stopped := approvedCoachWithServices(t, h, []string{"competition_support"})
		execSQLArgs(t, `INSERT INTO support_consult_settings (coach_id, accepting) VALUES ($1, false)`, stoppedCoach)
		if out := getApplies(t, stopped); out.Applies {
			t.Fatalf("not accepting applies should be false")
		}

		// 細目3：受付設定の行が無ければ担当サービスだけでは対象外。
		for _, services := range [][]string{{"competition_support"}, {"in_person"}} {
			_, tok := approvedCoachWithServices(t, h, services)
			if out := getApplies(t, tok); out.Applies {
				t.Fatalf("services %v without settings applies should be false", services)
			}
		}
	})

	t.Run("applies via published plans", func(t *testing.T) {
		// プラン作成ゲートが身元登録済みを要求するため、登録済みスタブを使う
		// （applies の判定だけを確かめ、registered は検証しない）。
		stub := identityStub(t, http.StatusNoContent)
		h := setupConfig(t, func(cfg *config.Config) {
			cfg.IdentityReaderURL = stub.URL
			cfg.IdentityReaderToken = "synthetic-read-only-identity-token"
		})
		// 受付設定なし＋公開状態・種別・入れ物だけを変えたプランで applies を確かめる。
		coachWithPlan := func(t *testing.T, serviceType string, published, consultTemplate bool) string {
			t.Helper()
			coach, tok := approvedCoachWithServices(t, h, []string{"video_review"})
			if rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/pledge", tok, map[string]any{"agree": true}); rec.Code != http.StatusOK {
				t.Fatalf("pledge %d %s", rec.Code, rec.Body.String())
			}
			if rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tok, planBody()); rec.Code != http.StatusCreated {
				t.Fatalf("create plan %d %s", rec.Code, rec.Body.String())
			}
			execSQLArgs(t, `UPDATE coaching_plans SET service_type=$2, published=$3, consult_template=$4 WHERE coach_id=$1`, coach, serviceType, published, consultTemplate)
			return tok
		}
		getApplies := func(t *testing.T, tok string) bool {
			t.Helper()
			rec := doJSON(t, h, http.MethodGet, "/v1/coach/me/identity-status", tok, nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d %s", rec.Code, rec.Body.String())
			}
			return decodeIdentityStatus(t, rec.Body.Bytes()).Applies
		}

		// 細目4・5：受付OFF/行なし＋公開中の in_person / competition_support プランは対象。
		for _, serviceType := range []string{"in_person", "competition_support"} {
			if applies := getApplies(t, coachWithPlan(t, serviceType, true, false)); !applies {
				t.Fatalf("published %s plan applies should be true", serviceType)
			}
		}
		// 細目6：下書き（非公開）だけでは対象外。
		for _, serviceType := range []string{"in_person", "competition_support"} {
			if applies := getApplies(t, coachWithPlan(t, serviceType, false, false)); applies {
				t.Fatalf("draft %s plan applies should be false", serviceType)
			}
		}
		// 細目7：公開中でも相談の入れ物プランだけでは対象外。
		if applies := getApplies(t, coachWithPlan(t, "competition_support", true, true)); applies {
			t.Fatalf("consult template plan applies should be false")
		}
	})

	// 細目9：registered の従来挙動は不変。そのいずれでも applies の計算は行われる。
	t.Run("unknown when unconfigured or service error", func(t *testing.T) {
		h := setupConfig(t, func(cfg *config.Config) {
			cfg.IdentityReaderURL = ""
			cfg.IdentityReaderToken = ""
		})
		coach, tok := approvedCoachWithServices(t, h, []string{"video_review"})
		execSQLArgs(t, `INSERT INTO support_consult_settings (coach_id, accepting) VALUES ($1, true)`, coach)
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
		coach, tok = approvedCoachWithServices(t, h, []string{"video_review"})
		execSQLArgs(t, `INSERT INTO support_consult_settings (coach_id, accepting) VALUES ($1, true)`, coach)
		rec = doJSON(t, h, http.MethodGet, "/v1/coach/me/identity-status", tok, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("service error %d %s", rec.Code, rec.Body.String())
		}
		if out := decodeIdentityStatus(t, rec.Body.Bytes()); out.Registered != nil {
			t.Fatalf("service error registered should be null (%s)", rec.Body.String())
		} else if !out.Applies {
			t.Fatalf("service error applies should still be true (%s)", rec.Body.String())
		}
	})

	// 細目9の一部：金庫に登録済み（204）のときは applies=true のケースでも registered=true。
	t.Run("registered true with applies", func(t *testing.T) {
		stub := identityStub(t, http.StatusNoContent)
		h := setupConfig(t, func(cfg *config.Config) {
			cfg.IdentityReaderURL = stub.URL
			cfg.IdentityReaderToken = "synthetic-read-only-identity-token"
		})
		coach, tok := approvedCoachWithServices(t, h, []string{"video_review"})
		execSQLArgs(t, `INSERT INTO support_consult_settings (coach_id, accepting) VALUES ($1, true)`, coach)
		rec := doJSON(t, h, http.MethodGet, "/v1/coach/me/identity-status", tok, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		out := decodeIdentityStatus(t, rec.Body.Bytes())
		if out.Registered == nil || !*out.Registered {
			t.Fatalf("registered = %v, want true (%s)", out.Registered, rec.Body.String())
		}
		if !out.Applies {
			t.Fatalf("applies should be true (%s)", rec.Body.String())
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
