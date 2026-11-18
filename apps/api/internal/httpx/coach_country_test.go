package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// コーチ一覧の受付状態と国旗（本体 #932）。
// 公開 API に on_duty と country_code を出し、国は運営だけが管理用 API で設定する。

// publicCoachesByID は、GET /v1/coaches の各行を id ごとに返す。
func publicCoachesByID(t *testing.T, h http.Handler) map[string]map[string]any {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("coaches %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Coaches []map[string]any `json:"coaches"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	m := map[string]map[string]any{}
	for _, c := range out.Coaches {
		m[c["id"].(string)] = c
	}
	return m
}

func publicCoach(t *testing.T, h http.Handler, id uuid.UUID) map[string]any {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coaches/"+id.String(), "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("coach %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func coachCountry(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var s string
	queryRowArgs(t, `SELECT country_code FROM coach_profiles WHERE profile_id = $1`, []any{id}, &s)
	return s
}

func adminApplicationCountries(t *testing.T, h http.Handler, tadmin string) map[string]any {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/admin/coach-applications?status=approved&per_page=100", tadmin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("applications %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Applications []map[string]any `json:"applications"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	m := map[string]any{}
	for _, a := range out.Applications {
		v, ok := a["country_code"]
		if !ok {
			t.Fatalf("country_code missing in application %v", a)
		}
		m[a["profile_id"].(string)] = v
	}
	return m
}

func TestCoachCountryAndOnDuty(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	onID, ton := onDutyCoach(t, h, tadmin)
	offID, _ := approvedReviewCoach(t, h, tadmin)

	// A1: 一覧と単体に on_duty（DB の値どおり）と country_code（初期値 JP）が出る。
	list := publicCoachesByID(t, h)
	for _, c := range []struct {
		id   uuid.UUID
		duty bool
	}{{onID, true}, {offID, false}} {
		row, ok := list[c.id.String()]
		if !ok {
			t.Fatalf("coach %s not listed (off duty coaches must stay listed)", c.id)
		}
		if row["on_duty"] != c.duty || row["country_code"] != "JP" {
			t.Fatalf("list row %s: on_duty=%v country_code=%v", c.id, row["on_duty"], row["country_code"])
		}
		one := publicCoach(t, h, c.id)
		if one["on_duty"] != c.duty || one["country_code"] != "JP" {
			t.Fatalf("detail %s: on_duty=%v country_code=%v", c.id, one["on_duty"], one["country_code"])
		}
	}

	url := "/v1/admin/coaching/coaches/" + offID.String() + "/country"

	// A3: 不正な国コードは 400（フィールドエラーの形）で、値は変わらない。
	for _, bad := range []string{"USA", "U1", "", "  "} {
		rec := doJSON(t, h, http.MethodPost, url, tadmin, map[string]any{"country_code": bad})
		var body struct {
			Error struct {
				Code   string            `json:"code"`
				Fields map[string]string `json:"fields"`
			} `json:"error"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if rec.Code != http.StatusBadRequest || body.Error.Code != "validation_error" || body.Error.Fields["country_code"] == "" {
			t.Fatalf("bad %q: %d %s", bad, rec.Code, rec.Body.String())
		}
		if got := coachCountry(t, offID); got != "JP" {
			t.Fatalf("changed by invalid %q: %s", bad, got)
		}
	}
	// 存在しないコーチは 404。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coaches/"+uuid.NewString()+"/country", tadmin, map[string]any{"country_code": "US"}); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown coach %d %s", rec.Code, rec.Body.String())
	}
	// 管理者でなければ設定できない（コーチ本人は 403、未ログインは 401）。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coaches/"+onID.String()+"/country", ton, map[string]any{"country_code": "US"}); rec.Code != http.StatusForbidden {
		t.Fatalf("coach self %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, url, "", map[string]any{"country_code": "US"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous %d %s", rec.Code, rec.Body.String())
	}
	if coachCountry(t, onID) != "JP" || coachCountry(t, offID) != "JP" {
		t.Fatal("changed by non-admin")
	}

	// A2: 管理者は設定できる。小文字・前後の空白も大文字の2字として受ける。
	rec := doJSON(t, h, http.MethodPost, url, tadmin, map[string]any{"country_code": " us "})
	if rec.Code != http.StatusOK {
		t.Fatalf("set %d %s", rec.Code, rec.Body.String())
	}
	var set struct {
		Application map[string]any `json:"application"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &set); err != nil || set.Application["country_code"] != "US" {
		t.Fatalf("set body %s", rec.Body.String())
	}
	list = publicCoachesByID(t, h)
	if list[offID.String()]["country_code"] != "US" || list[onID.String()]["country_code"] != "JP" {
		t.Fatalf("list after set: off=%v on=%v", list[offID.String()]["country_code"], list[onID.String()]["country_code"])
	}
	if got := publicCoach(t, h, offID)["country_code"]; got != "US" {
		t.Fatalf("detail after set %v", got)
	}
	// 国の設定で受付状態は変わらない。
	if list[offID.String()]["on_duty"] != false || list[onID.String()]["on_duty"] != true {
		t.Fatal("on_duty changed by country update")
	}

	// 状態は問わない：承認前の応募者にも設定できる。
	pending := uuid.New()
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", token(t, pending), coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coaches/"+pending.String()+"/country", tadmin, map[string]any{"country_code": "KE"}); rec.Code != http.StatusOK {
		t.Fatalf("pending %d %s", rec.Code, rec.Body.String())
	}
	if got := coachCountry(t, pending); got != "KE" {
		t.Fatalf("pending country %s", got)
	}

	// A4: 管理画面の応募一覧の各行に country_code が出る。
	countries := adminApplicationCountries(t, h, tadmin)
	if countries[offID.String()] != "US" || countries[onID.String()] != "JP" {
		t.Fatalf("admin list countries %v", countries)
	}

	// A5: コーチ本人は GET /coach/me で読めるが、応募・プロフィール更新では変えられない。
	toff := token(t, offID)
	me := doJSON(t, h, http.MethodGet, "/v1/coach/me", toff, nil)
	var meOut struct {
		Application map[string]any `json:"application"`
	}
	if err := json.Unmarshal(me.Body.Bytes(), &meOut); err != nil || me.Code != http.StatusOK || meOut.Application["country_code"] != "US" {
		t.Fatalf("coach me %d %s", me.Code, me.Body.String())
	}
	upd := profileUpdateBody()
	upd["country_code"] = "JP"
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/profile", toff, upd); rec.Code != http.StatusBadRequest {
		t.Fatalf("profile update with country_code %d %s", rec.Code, rec.Body.String())
	}
	app := coachApplicationBody()
	app["country_code"] = "FR"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", token(t, uuid.New()), app); rec.Code != http.StatusBadRequest {
		t.Fatalf("application with country_code %d %s", rec.Code, rec.Body.String())
	}
	// 受付の切り替え（PATCH /coach/me）でも国は変わらず、応答にも設定値が出る。
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", toff, map[string]any{"on_duty": false}); rec.Code != http.StatusOK || !jsonHas(rec.Body.Bytes(), "US") {
		t.Fatalf("duty toggle %d %s", rec.Code, rec.Body.String())
	}
	if got := coachCountry(t, offID); got != "US" {
		t.Fatalf("country after self edits %s", got)
	}
}

// jsonHas は、応答の application.country_code が want かを見る。
func jsonHas(b []byte, want string) bool {
	var out struct {
		Application map[string]any `json:"application"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return false
	}
	return out.Application["country_code"] == want
}

// A6: 既存のコーチ行は JP で埋まり、CHECK 制約で不正な値は入らない。
func TestCoachCountryMigration(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	id, _ := approvedReviewCoach(t, h, tadmin)

	if n := scalar(t, `SELECT count(*) FROM coach_profiles WHERE country_code <> 'JP'`); n != 0 {
		t.Fatalf("non-JP rows after migration: %d", n)
	}
	if n := scalar(t, `SELECT count(*) FROM information_schema.columns WHERE table_name = 'coach_profiles' AND column_name = 'country_code' AND is_nullable = 'NO' AND column_default LIKE '''JP''%'`); n != 1 {
		t.Fatalf("column definition mismatch: %d", n)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, bad := range []string{"JPN", "jp", "J1", ""} {
		if _, err := pool.Exec(ctx, `UPDATE coach_profiles SET country_code = $2 WHERE profile_id = $1`, id, bad); err == nil {
			t.Fatalf("CHECK accepted %q", bad)
		}
	}
	if got := coachCountry(t, id); got != "JP" {
		t.Fatalf("country %s", got)
	}
}
