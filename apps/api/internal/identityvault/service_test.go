package identityvault

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/auth"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/migrate"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIdentityIsolation(t *testing.T) {
	ctx := context.Background()
	main, vault := testDatabases(t)
	coach, coachSubject, staff, outsider, plan := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	execTest(t, main, "INSERT INTO profiles(id) VALUES ($1)", coach)
	execTest(t, main, `INSERT INTO coach_profiles
		(profile_id,tier,events,best_record,achievements,years,status,reviewed_at,pledged_at,pledge_version)
		VALUES ($1,'coach',ARRAY['100m'],'test','test',1,'approved',now(),now(),1)`, coach)
	execTest(t, main, `INSERT INTO coaching_plans
		(id,coach_id,kind,title,description,duration_days,price_yen,capacity,published)
		VALUES ($1,$2,'period','test','test',30,8000,2,true)`, plan, coach)
	execTest(t, vault, "INSERT INTO identity_vault.members(subject,coach_id,is_admin) VALUES ($1,$2,false),($3,NULL,true)", coachSubject, coach, staff)
	readerRole := "identity_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	execTest(t, main, "CREATE ROLE "+readerRole)
	t.Cleanup(func() {
		execTest(t, main, "DROP OWNED BY "+readerRole)
		execTest(t, main, "DROP ROLE "+readerRole)
	})
	execTest(t, main, "GRANT USAGE ON SCHEMA identity_boundary TO "+readerRole)
	execTest(t, main, "GRANT SELECT ON identity_boundary.published_plans,identity_boundary.approved_coaches,identity_boundary.retention TO "+readerRole)
	execTest(t, main, "GRANT EXECUTE ON FUNCTION identity_boundary.last_transaction(uuid) TO "+readerRole)
	// 終了記録の実行権限だけは、配備時に vault 用ロールへ個別に付与する想定で、
	// 試験の読取ロールにも同じ付与を再現する（公開 API の実行ロールには付与しない）。
	execTest(t, main, "GRANT EXECUTE ON FUNCTION identity_boundary.mark_retired(uuid) TO "+readerRole)
	readConfig := main.Config()
	readConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE "+readerRole)
		return err
	}
	readDB, err := pgxpool.NewWithConfig(ctx, readConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(readDB.Close)
	if _, err := readDB.Exec(ctx, "UPDATE identity_boundary.plan_epochs SET epoch=gen_random_uuid()"); err == nil {
		t.Fatal("read role can rewrite a publication epoch")
	}
	if _, err := readDB.Exec(ctx, "SELECT * FROM identity_vault.identities"); err == nil {
		t.Fatal("main database can read vault records")
	}

	publicRole := "public_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	execTest(t, main, "CREATE ROLE "+publicRole+" BYPASSRLS")
	t.Cleanup(func() {
		execTest(t, main, "DROP OWNED BY "+publicRole)
		execTest(t, main, "DROP ROLE "+publicRole)
	})
	execTest(t, main, "GRANT SELECT,UPDATE(published) ON coaching_plans TO "+publicRole)
	execTest(t, main, "GRANT SELECT,UPDATE(status) ON coach_profiles TO "+publicRole)
	publicConfig := main.Config()
	publicConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE "+publicRole)
		return err
	}
	publicDB, err := pgxpool.NewWithConfig(ctx, publicConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(publicDB.Close)
	for _, forbidden := range []string{
		"UPDATE identity_boundary.plan_epochs SET epoch=gen_random_uuid()",
		"ALTER TABLE coaching_plans DISABLE TRIGGER identity_plan_change",
		"TRUNCATE coaching_plans CASCADE",
		"SET session_replication_role = replica",
		"SELECT * FROM identity_vault.identities",
		"SELECT identity_boundary.mark_retired(gen_random_uuid())",
		"SELECT * FROM identity_boundary.retirement",
	} {
		if _, err := publicDB.Exec(ctx, forbidden); err == nil {
			t.Fatalf("public runtime accepted forbidden statement: %s", forbidden)
		}
	}

	signing, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/v1/.well-known/jwks.json" {
			encode := base64.RawURLEncoding.EncodeToString
			respond(w, map[string]any{"keys": []map[string]string{{
				"kty": "EC", "crv": "P-256", "alg": "ES256", "kid": "vault-test",
				"x": encode(signing.X.FillBytes(make([]byte, 32))), "y": encode(signing.Y.FillBytes(make([]byte, 32))),
			}}})
			return
		}
		if r.URL.Path == "/auth/v1/user" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(authServer.Close)
	verifier, err := auth.NewVerifier("", authServer.URL, "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	makeToken := func(subject uuid.UUID, aal, issuer string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
			"sub": subject.String(), "aal": aal, "iss": issuer, "aud": "authenticated",
			"session_id": uuid.NewString(), "exp": time.Now().Add(time.Hour).Unix(),
		})
		token.Header["kid"] = "vault-test"
		signed, err := token.SignedString(signing)
		if err != nil {
			t.Fatal(err)
		}
		return signed
	}
	issuer := authServer.URL + "/auth/v1"
	coachToken := makeToken(coachSubject, "aal2", issuer)
	staffToken := makeToken(staff, "aal2", issuer)
	machine := strings.Repeat("r", 32)
	cleanup := strings.Repeat("c", 32)
	end := strings.Repeat("e", 32)
	svc, err := New(readDB, vault, verifier, auth.Admin{BaseURL: authServer.URL, APIKey: "synthetic-publishable"}, base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{5}, 32)), machine, cleanup, end, "https://example.test")
	if err != nil {
		t.Fatal(err)
	}
	handler := svc.Handler()
	request := func(method, path, authToken, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+authToken)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != status {
			t.Fatalf("%s %s: got %d, want %d: %s", method, path, res.Code, status, res.Body.String())
		}
		return res
	}

	const syntheticIdentity = `{"legal_name":"合成テスト氏名","address":"合成テスト住所"}`
	const viewCoach = "/v1/coach"
	const save = "/v1/coach/identity"
	adminView := "/v1/admin/coaches/" + coach.String() + "/identity"
	publish := "/v1/coach/disclosures/" + plan.String()
	public := "/internal/disclosures/" + plan.String()
	request("GET", viewCoach, machine, "", 401)
	request("PUT", save, makeToken(coachSubject, "aal2", "https://public.example.test/auth/v1"), syntheticIdentity, 401)
	request("POST", adminView, makeToken(staff, "aal1", issuer), `{"purpose":"test"}`, 403)
	request("POST", adminView, coachToken, `{"purpose":"test"}`, 403)
	request("GET", viewCoach, makeToken(outsider, "aal2", issuer), "", 403)
	request("PUT", publish, machine, `{"confirm":true}`, 401)
	request("POST", "/internal/cleanup", machine, "", 401)
	request("GET", public, machine, "", 404)
	endPath := "/internal/coach-end"
	request("POST", endPath, machine, `{"coach_id":"`+coach.String()+`"}`, 401)
	request("GET", public, end, "", 401)
	request("POST", "/internal/cleanup", end, "", 401)
	ended := uuid.New()
	execTest(t, main, "INSERT INTO profiles(id) VALUES ($1)", ended)
	execTest(t, main, `INSERT INTO coach_profiles
		(profile_id,tier,events,best_record,achievements,years,status,reviewed_at,pledged_at,pledge_version)
		VALUES ($1,'coach',ARRAY['100m'],'test','test',1,'rejected',now(),now(),1)`, ended)
	retired := func() int {
		t.Helper()
		var n int
		if err := main.QueryRow(ctx, "SELECT count(*) FROM identity_boundary.retirement WHERE coach_id=$1", ended).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	request("POST", endPath, end, "", 400)
	request("POST", endPath, end, `{"coach_id":"not-a-uuid"}`, 400)
	request("POST", endPath, end, `{"coach_id":"`+ended.String()+`"}`, 204)
	if retired() != 1 {
		t.Fatal("rejected coach end was not recorded")
	}
	request("POST", endPath, end, `{"coach_id":"`+ended.String()+`"}`, 204)
	if retired() != 1 {
		t.Fatal("end recording was applied twice")
	}
	execTest(t, main, "UPDATE coach_profiles SET status='approved' WHERE profile_id=$1", ended)
	request("POST", endPath, end, `{"coach_id":"`+ended.String()+`"}`, 409)
	if retired() != 0 {
		t.Fatal("approved coach end was recorded")
	}
	unknown := uuid.New()
	request("POST", endPath, end, `{"coach_id":"`+unknown.String()+`"}`, 204)
	execTest(t, main, "DELETE FROM identity_boundary.retirement WHERE coach_id=$1", unknown)
	request("PUT", save, coachToken, `{"legal_name":"test","address":"test","phone":"0000000000"}`, 400)
	request("PUT", save, coachToken, syntheticIdentity, 204)

	var sealed []byte
	if err := vault.QueryRow(ctx, "SELECT address_enc FROM identity_vault.identities WHERE coach_id=$1", coach).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("合成テスト住所")) {
		t.Fatal("plaintext stored in private identity table")
	}
	if _, err := svc.open(uuid.New(), "address", sealed); err == nil {
		t.Fatal("ciphertext substitution was accepted")
	}
	private := request("GET", viewCoach, coachToken, "", 200)
	var own struct {
		Revision uuid.UUID `json:"identity_revision"`
	}
	if err := json.Unmarshal(private.Body.Bytes(), &own); err != nil || own.Revision == uuid.Nil || !strings.Contains(private.Body.String(), "合成テスト住所") {
		t.Fatal("owner did not get registered identity")
	}
	admin := request("POST", adminView, staffToken, `{"purpose":"開示請求の合成試験"}`, 200)
	if !strings.Contains(admin.Body.String(), "合成テスト住所") || !strings.Contains(admin.Body.String(), staff.String()) ||
		!strings.Contains(admin.Body.String(), "開示請求の合成試験") || strings.Contains(admin.Body.String(), "phone") {
		t.Fatal("administrator disclosure or audit record did not match")
	}

	consent := `{"confirm":true,"phone":"000-0000-0000","identity_revision":"` + own.Revision.String() + `"}`
	request("GET", public, machine, "", 404)
	request("PUT", publish, coachToken, consent, 204)
	if plans := request("GET", "/internal/disclosures?ids="+plan.String(), machine, "", 200); !strings.Contains(plans.Body.String(), plan.String()) {
		t.Fatal("approved plan was hidden from the plan list")
	}
	res := request("GET", public, machine, "", 200)
	if !strings.Contains(res.Body.String(), "合成テスト住所") || !strings.Contains(res.Body.String(), "000-0000-0000") || res.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("public disclosure did not match the approved data")
	}
	execTest(t, publicDB, "UPDATE coaching_plans SET published=false WHERE id=$1", plan)
	request("GET", public, machine, "", 404)
	execTest(t, publicDB, "UPDATE coaching_plans SET published=true WHERE id=$1", plan)
	request("GET", public, machine, "", 404)
	request("PUT", publish, coachToken, consent, 204)
	execTest(t, publicDB, "UPDATE coach_profiles SET status='suspended' WHERE profile_id=$1", coach)
	request("GET", public, machine, "", 404)
	execTest(t, publicDB, "UPDATE coach_profiles SET status='approved' WHERE profile_id=$1", coach)
	request("GET", public, machine, "", 404)
	request("PUT", publish, coachToken, consent, 204)
	request("PUT", "/v1/coach/exemption", coachToken, `{"exempt":true,"checklist_version":1,"checked":[true,true,true]}`, 204)
	request("GET", public, machine, "", 404)
	request("PUT", publish, coachToken, consent, 409)
	request("PUT", publish, coachToken, `{"confirm":true,"identity_revision":"`+own.Revision.String()+`"}`, 204)
	if out := request("GET", public, machine, "", 200); strings.TrimSpace(out.Body.String()) != `{"required":false}` {
		t.Fatal("exempt plan exposed fields")
	}
	request("PUT", save, coachToken, syntheticIdentity, 204)
	request("GET", public, machine, "", 404)
	request("PUT", publish, coachToken, `{"confirm":true,"identity_revision":"`+own.Revision.String()+`"}`, 409)

	execTest(t, publicDB, "UPDATE coach_profiles SET status='withdrawn' WHERE profile_id=$1", coach)
	request("POST", "/internal/cleanup", cleanup, "", 204)
	request("POST", adminView, staffToken, `{"purpose":"retention test"}`, 200)
	execTest(t, main, "UPDATE identity_boundary.retirement SET ended_at=now()-interval '6 years',last_transaction_at=now()-interval '4 years' WHERE coach_id=$1", coach)
	request("POST", "/internal/cleanup", cleanup, "", 204)
	request("POST", adminView, staffToken, `{"purpose":"still within retention"}`, 200)
	execTest(t, main, "UPDATE identity_boundary.retirement SET last_transaction_at=now()-interval '6 years' WHERE coach_id=$1", coach)
	execTest(t, vault, "UPDATE identity_vault.identities SET last_transaction_at=now()-interval '6 years' WHERE coach_id=$1", coach)
	request("POST", "/internal/cleanup", cleanup, "", 204)
	request("POST", adminView, staffToken, `{"purpose":"retention expired"}`, 404)
}

func execTest(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func testDatabases(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("set DATABASE_URL to a local test server with database creation rights")
	}
	if err := config.RequireLocalDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	host, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(host.Close)
	newDB := func(prefix string) (string, *pgxpool.Pool) {
		name := prefix + strings.ReplaceAll(uuid.NewString(), "-", "")
		execTest(t, host, "CREATE DATABASE "+name)
		t.Cleanup(func() { execTest(t, host, "DROP DATABASE "+name) })
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		u.Path = "/" + name
		pool, err := pgxpool.New(ctx, u.String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		return u.String(), pool
	}
	mainDSN, main := newDB("main_test_")
	if err := migrate.Up(mainDSN); err != nil {
		t.Fatal(err)
	}
	_, vault := newDB("vault_test_")
	schema, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "deploy", "identity-vault", "schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	execTest(t, vault, string(schema))
	return main, vault
}
