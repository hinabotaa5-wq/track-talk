package identityvault

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/auth"
)

// TestOperatorRequiredFlipNeedsCoachConfirmation は、運営者が「表記しない」の
// コーチを「表記要」に戻したとき、預かっている氏名・住所や過去の承認時の
// 電話番号が、本人の確認なしに公開側（公開 API が読む内部経路）へ出ないことを
// 確かめる（本体 #432 の手順1）。データはすべて合成。
func TestOperatorRequiredFlipNeedsCoachConfirmation(t *testing.T) {
	main, vault := testDatabases(t)
	coach, coachSubject, staff, plan := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	execTest(t, main, "INSERT INTO profiles(id) VALUES ($1)", coach)
	execTest(t, main, `INSERT INTO coach_profiles
		(profile_id,tier,events,best_record,achievements,years,status,reviewed_at,pledged_at,pledge_version)
		VALUES ($1,'coach',ARRAY['100m'],'test','test',1,'approved',now(),now(),1)`, coach)
	execTest(t, main, `INSERT INTO coaching_plans
		(id,coach_id,kind,title,description,duration_days,price_yen,capacity,published)
		VALUES ($1,$2,'period','test','test',30,8000,2,true)`, plan, coach)
	execTest(t, vault, "INSERT INTO identity_vault.members(subject,coach_id,is_admin) VALUES ($1,$2,false),($3,NULL,true)", coachSubject, coach, staff)

	signing, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/v1/.well-known/jwks.json":
			encode := base64.RawURLEncoding.EncodeToString
			respond(w, map[string]any{"keys": []map[string]string{{
				"kty": "EC", "crv": "P-256", "alg": "ES256", "kid": "vault-test",
				"x": encode(signing.X.FillBytes(make([]byte, 32))), "y": encode(signing.Y.FillBytes(make([]byte, 32))),
			}}})
		case "/auth/v1/user":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(authServer.Close)
	verifier, err := auth.NewVerifier("", authServer.URL, "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	issuer := authServer.URL + "/auth/v1"
	makeToken := func(subject uuid.UUID) string {
		token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
			"sub": subject.String(), "aal": "aal2", "iss": issuer, "aud": "authenticated",
			"session_id": uuid.NewString(), "exp": time.Now().Add(time.Hour).Unix(),
		})
		token.Header["kid"] = "vault-test"
		signed, err := token.SignedString(signing)
		if err != nil {
			t.Fatal(err)
		}
		return signed
	}
	coachToken, staffToken := makeToken(coachSubject), makeToken(staff)
	machine := strings.Repeat("r", 32)
	svc, err := New(main, vault, verifier, auth.Admin{BaseURL: authServer.URL, APIKey: "synthetic-publishable"},
		base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)), machine, strings.Repeat("c", 32), strings.Repeat("e", 32), "https://example.test")
	if err != nil {
		t.Fatal(err)
	}
	handler := svc.Handler()
	request := func(method, path, authToken, body string, status int) string {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+authToken)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != status {
			t.Fatalf("%s %s: got %d, want %d: %s", method, path, res.Code, status, res.Body.String())
		}
		return res.Body.String()
	}
	type coachView struct {
		Required bool      `json:"required"`
		Forced   bool      `json:"forced"`
		Revision uuid.UUID `json:"identity_revision"`
		Identity *struct{} `json:"identity"`
		Plans    []struct {
			ID         uuid.UUID `json:"id"`
			Authorized bool      `json:"authorized"`
		} `json:"plans"`
	}
	view := func() coachView {
		t.Helper()
		var v coachView
		if err := json.Unmarshal([]byte(request("GET", "/v1/coach", coachToken, "", 200)), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	public := "/internal/disclosures/" + plan.String()
	list := "/internal/disclosures?ids=" + plan.String()
	publish := "/v1/coach/disclosures/" + plan.String()
	force := "/v1/admin/coaches/" + coach.String() + "/require-disclosure"
	const name, address, phone = "合成フリップ氏名", "合成フリップ住所", "000-1111-2222"
	listed := func() bool {
		t.Helper()
		return strings.Contains(request("GET", list, machine, "", 200), plan.String())
	}
	leaks := func(body string) bool {
		return strings.Contains(body, name) || strings.Contains(body, address) || strings.Contains(body, phone)
	}

	// 前提: 身元を登録し、一度は表記要で電話つき承認をしたあと、「表記しない」を自己申告して公開中。
	request("PUT", "/v1/coach/identity", coachToken, `{"legal_name":"`+name+`","address":"`+address+`"}`, 204)
	rev := view().Revision
	request("PUT", publish, coachToken, `{"confirm":true,"phone":"`+phone+`","identity_revision":"`+rev.String()+`"}`, 204)
	if !leaks(request("GET", public, machine, "", 200)) {
		t.Fatal("required approval did not publish confirmed fields")
	}
	request("PUT", "/v1/coach/exemption", coachToken, `{"exempt":true,"checklist_version":1,"checked":[true,true,true]}`, 204)
	request("PUT", publish, coachToken, `{"confirm":true,"identity_revision":"`+rev.String()+`"}`, 204)
	if body := request("GET", public, machine, "", 200); strings.TrimSpace(body) != `{"required":false}` || !listed() {
		t.Fatalf("exempt plan is not published without fields: %s", body)
	}

	// 運営者が「表記要」に戻した直後: 公開経路は項目を返さず、プランは公開対象から外れる。
	request("POST", force, staffToken, "", 204)
	if body := request("GET", public, machine, "", 404); leaks(body) {
		t.Fatal("fields leaked right after operator flip")
	}
	if listed() {
		t.Fatal("plan stayed listed after operator flip")
	}
	var grants int
	if err := vault.QueryRow(t.Context(), "SELECT count(*) FROM identity_vault.disclosures WHERE coach_id=$1", coach).Scan(&grants); err != nil || grants != 0 {
		t.Fatalf("approvals survived the operator flip: %d %v", grants, err)
	}
	v := view()
	if !v.Required || !v.Forced || v.Identity == nil || len(v.Plans) != 1 || v.Plans[0].Authorized {
		t.Fatalf("coach view after flip: %+v", v)
	}

	// 本人の確認なしでは戻らない: 再申告・電話なし・古い版の承認はすべて拒否。
	request("PUT", "/v1/coach/exemption", coachToken, `{"exempt":true,"checklist_version":1,"checked":[true,true,true]}`, 409)
	request("PUT", publish, coachToken, `{"confirm":true,"identity_revision":"`+v.Revision.String()+`"}`, 409)
	request("PUT", publish, coachToken, `{"confirm":true,"phone":"`+phone+`","identity_revision":"`+uuid.NewString()+`"}`, 409)
	request("PUT", publish, coachToken, `{"confirm":false,"phone":"`+phone+`","identity_revision":"`+v.Revision.String()+`"}`, 422)
	request("GET", public, machine, "", 404)

	// 本人が掲載内容（現在の版の氏名・住所＋この時点で入れた電話）を確認・承認すると再公開。
	request("PUT", publish, coachToken, `{"confirm":true,"phone":"`+phone+`","identity_revision":"`+v.Revision.String()+`"}`, 204)
	if body := request("GET", public, machine, "", 200); !leaks(body) || !listed() {
		t.Fatalf("confirmed fields were not published: %s", body)
	}

	// 表記要のまま運営者が再度切り替えても、承認はやり直しになる。
	request("POST", force, staffToken, "", 204)
	request("GET", public, machine, "", 404)
	request("PUT", publish, coachToken, `{"confirm":true,"phone":"`+phone+`","identity_revision":"`+v.Revision.String()+`"}`, 204)

	// 登録情報の更新・退会でも、確認済みの承認は効かなくなる。
	request("PUT", "/v1/coach/identity", coachToken, `{"legal_name":"`+name+`","address":"`+address+`"}`, 204)
	request("GET", public, machine, "", 404)
	request("PUT", publish, coachToken, `{"confirm":true,"phone":"`+phone+`","identity_revision":"`+view().Revision.String()+`"}`, 204)
	request("GET", public, machine, "", 200)
	execTest(t, main, "UPDATE coach_profiles SET status='withdrawn' WHERE profile_id=$1", coach)
	request("GET", public, machine, "", 404)
	if listed() {
		t.Fatal("withdrawn coach plan stayed listed")
	}
}
