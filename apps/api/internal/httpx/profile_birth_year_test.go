package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type birthYearMe struct {
	BirthYear      *int  `json:"birth_year"`
	BirthYearKnown *bool `json:"birth_year_known"`
}

func readBirthYearMe(t *testing.T, h http.Handler, tok string) birthYearMe {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/me", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/me %d %s", rec.Code, rec.Body.String())
	}
	var out birthYearMe
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.BirthYearKnown == nil {
		t.Fatalf("GET /v1/me body %s %v", rec.Body.String(), err)
	}
	return out
}

// 生まれた年はプロフィールの保存で一度だけ登録する（本体 #1045 の T1・T2・T3・T5）。
func TestProfileBirthYearRegistration(t *testing.T) {
	h := setup(t)
	tok := token(t, uuid.New())
	if me := readBirthYearMe(t, h, tok); me.BirthYear != nil || *me.BirthYearKnown {
		t.Fatalf("new account: %+v", me)
	}

	thisYear := time.Now().In(time.FixedZone("JST", 9*60*60)).Year()
	base := map[string]any{"nickname": "はしる", "nickname_public": false, "event": "100m"}
	with := func(v any) map[string]any {
		body := map[string]any{}
		for k, x := range base {
			body[k] = x
		}
		if v != nil {
			body["birth_year"] = v
		}
		return body
	}
	// 未入力・未来の年・1900年より前・数字でない値・桁違いは、理由つきで止める。
	for _, bad := range []any{nil, "", thisYear + 1, 1899, "abcd", "20O8", 208, "20080"} {
		rec := doJSON(t, h, http.MethodPatch, "/v1/me", tok, with(bad))
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"birth_year"`) {
			t.Fatalf("birth_year %v: %d %s", bad, rec.Code, rec.Body.String())
		}
	}
	if me := readBirthYearMe(t, h, tok); me.BirthYear != nil || *me.BirthYearKnown {
		t.Fatalf("saved after invalid input: %+v", me)
	}

	// 文字列の4桁でも受け付け、GET /v1/me で返る。
	rec := doJSON(t, h, http.MethodPatch, "/v1/me", tok, with("2008"))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"birth_year":2008`) {
		t.Fatalf("register: %d %s", rec.Code, rec.Body.String())
	}
	if me := readBirthYearMe(t, h, tok); me.BirthYear == nil || *me.BirthYear != 2008 || !*me.BirthYearKnown {
		t.Fatalf("after register: %+v", me)
	}

	// 登録したあとは、送らなければそのまま保存でき、送り直すと 409（本人は変えられない）。
	if rec := doJSON(t, h, http.MethodPatch, "/v1/me", tok, with(nil)); rec.Code != http.StatusOK {
		t.Fatalf("save without birth_year: %d %s", rec.Code, rec.Body.String())
	}
	for _, again := range []any{2008, 1990} {
		rec := doJSON(t, h, http.MethodPatch, "/v1/me", tok, with(again))
		if rec.Code != http.StatusConflict {
			t.Fatalf("re-register %v: %d %s", again, rec.Code, rec.Body.String())
		}
	}
	if me := readBirthYearMe(t, h, tok); me.BirthYear == nil || *me.BirthYear != 2008 {
		t.Fatalf("changed after 409: %+v", me)
	}

	// 1900年と今年は受け付ける（境界）。
	for _, ok := range []int{1900, thisYear} {
		other := token(t, uuid.New())
		if rec := doJSON(t, h, http.MethodPatch, "/v1/me", other, with(ok)); rec.Code != http.StatusOK {
			t.Fatalf("boundary %d: %d %s", ok, rec.Code, rec.Body.String())
		}
	}
}

// 旧方式で生まれた年月（birth_month）を登録した人は、年の情報ありとして聞き直さない（T4）。
// birth_year 列へは書き戻さず、送っても 409。
func TestProfileBirthYearKnownFromBirthMonth(t *testing.T) {
	h := setup(t)
	id := uuid.New()
	tok := token(t, id)
	readBirthYearMe(t, h, tok)
	execSQLArgs(t, `UPDATE profiles SET birth_month = '2005-04-01' WHERE id = $1`, id)
	if me := readBirthYearMe(t, h, tok); me.BirthYear != nil || !*me.BirthYearKnown {
		t.Fatalf("birth_month user: %+v", me)
	}
	body := map[string]any{"nickname": "はしる", "nickname_public": false, "event": "100m"}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/me", tok, body); rec.Code != http.StatusOK {
		t.Fatalf("save without birth_year: %d %s", rec.Code, rec.Body.String())
	}
	body["birth_year"] = 2005
	if rec := doJSON(t, h, http.MethodPatch, "/v1/me", tok, body); rec.Code != http.StatusConflict {
		t.Fatalf("birth_month user sends birth_year: %d %s", rec.Code, rec.Body.String())
	}
	var stored *int
	queryRowArgs(t, `SELECT birth_year FROM profiles WHERE id = $1`, []any{id}, &stored)
	if stored != nil {
		t.Fatalf("birth_year written back: %v", *stored)
	}
}
