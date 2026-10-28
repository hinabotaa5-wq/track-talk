package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// tokenWithAAL は、二段階認証の段階(aal1・aal2)を入れたトークンを作る。
func tokenWithAAL(t *testing.T, id uuid.UUID, aal string) string {
	t.Helper()
	secret := os.Getenv("SUPABASE_JWT_SECRET")
	if secret == "" {
		secret = "test-jwt-secret-for-ci-only"
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": id.String(),
		"aud": "authenticated",
		"exp": time.Now().Add(time.Hour).Unix(),
		"aal": aal,
	})
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func meMFARequired(t *testing.T, h http.Handler, tok string) bool {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/me", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("me %d %s", rec.Code, rec.Body.String())
	}
	var me struct {
		MFARequired bool `json:"mfa_required"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	return me.MFARequired
}

func TestStaffMFARequiredForAdminAndCoach(t *testing.T) {
	h := setupConfig(t, func(c *config.Config) { c.StaffMFARequired = true })

	admin := uuid.New()
	admin1, admin2 := tokenWithAAL(t, admin, "aal1"), tokenWithAAL(t, admin, "aal2")
	if meMFARequired(t, h, admin1) {
		t.Fatal("not admin yet")
	}
	setAdmin(t, admin)

	// 二段階認証を済ませていないログインでは、直に API を呼んでも管理の操作を拒む。
	rec := doJSON(t, h, http.MethodGet, "/v1/admin/stats", admin1, nil)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"mfa_required"`) {
		t.Fatalf("admin aal1 %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/stats", token(t, admin), nil); rec.Code != http.StatusForbidden {
		t.Fatalf("admin without aal %d", rec.Code)
	}
	if !meMFARequired(t, h, admin1) {
		t.Fatal("admin aal1 should be asked for mfa")
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/stats", admin2, nil); rec.Code != http.StatusOK {
		t.Fatalf("admin aal2 %d %s", rec.Code, rec.Body.String())
	}
	if meMFARequired(t, h, admin2) {
		t.Fatal("admin aal2 is done")
	}

	coach := uuid.New()
	coach1, coach2 := tokenWithAAL(t, coach, "aal1"), tokenWithAAL(t, coach, "aal2")
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", coach1, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	// 審査待ちのうちはコーチの操作ができないので、二段階認証も求めない。
	if meMFARequired(t, h, coach1) {
		t.Fatal("pending coach")
	}
	withCoachPhoto(t, coach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+coach.String()+"/approve", admin2, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/coach/me/plans", coach1, nil)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"mfa_required"`) {
		t.Fatalf("coach aal1 %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", coach1, map[string]any{"on_duty": true}); rec.Code != http.StatusForbidden {
		t.Fatalf("coach duty aal1 %d %s", rec.Code, rec.Body.String())
	}
	if !meMFARequired(t, h, coach1) {
		t.Fatal("approved coach aal1 should be asked for mfa")
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/coach/me/plans", coach2, nil); rec.Code != http.StatusOK {
		t.Fatalf("coach aal2 %d %s", rec.Code, rec.Body.String())
	}

	// ふつうの利用者には求めない。
	user := uuid.New()
	tu := tokenWithAAL(t, user, "aal1")
	if meMFARequired(t, h, tu) {
		t.Fatal("user")
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/stats", tu, nil); rec.Code != http.StatusForbidden || strings.Contains(rec.Body.String(), "mfa_required") {
		t.Fatalf("user admin %d %s", rec.Code, rec.Body.String())
	}
}

func TestStaffMFANotRequiredWhenOff(t *testing.T) {
	h := setup(t)
	admin := uuid.New()
	ta := tokenWithAAL(t, admin, "aal1")
	if meMFARequired(t, h, ta) {
		t.Fatal("me before admin")
	}
	setAdmin(t, admin)
	if meMFARequired(t, h, ta) {
		t.Fatal("off should not ask")
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/stats", ta, nil); rec.Code != http.StatusOK {
		t.Fatalf("admin %d %s", rec.Code, rec.Body.String())
	}
}

// 二段階認証を設定済みの人は、コードを入れる前のログイン(aal1)では API を使えない。
// 画面で止めるだけだと、パスワードだけ知った人がトークンで直に API を呼べてしまうため。
func TestMFAEnrolledUserNeedsAAL2(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	h := setup(t)
	// 本番の Supabase にある auth.mfa_factors の、使う列だけを作る。
	if _, err := pool.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS auth;
		CREATE TABLE IF NOT EXISTS auth.mfa_factors (id uuid PRIMARY KEY, user_id uuid NOT NULL, status text NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	enrolled, pending, plain := uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM auth.mfa_factors WHERE user_id = ANY($1)`, []uuid.UUID{enrolled, pending})
	})
	if _, err := pool.Exec(ctx, `INSERT INTO auth.mfa_factors (id, user_id, status) VALUES ($1, $2, 'verified'), ($3, $4, 'unverified')`,
		uuid.New(), enrolled, uuid.New(), pending); err != nil {
		t.Fatal(err)
	}

	rec := doJSON(t, h, http.MethodGet, "/v1/me", tokenWithAAL(t, enrolled, "aal1"), nil)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "mfa_required") {
		t.Fatalf("enrolled aal1 %d %s", rec.Code, rec.Body.String())
	}
	// ログインなしで読めるものは、ログインしていない人として読める。
	rec = doJSON(t, h, http.MethodGet, "/v1/posts", tokenWithAAL(t, enrolled, "aal1"), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("enrolled aal1 public %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me", tokenWithAAL(t, enrolled, "aal2"), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("enrolled aal2 %d %s", rec.Code, rec.Body.String())
	}
	// 設定の途中(コードを確かめていない)の人と、設定していない人は、これまでどおり使える。
	for _, id := range []uuid.UUID{pending, plain} {
		rec = doJSON(t, h, http.MethodGet, "/v1/me", tokenWithAAL(t, id, "aal1"), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("not enrolled aal1 %d %s", rec.Code, rec.Body.String())
		}
	}
}
