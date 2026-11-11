package identityvault

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
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

// captureLogs は試験中の標準ログを集める。身元情報の平文がログに出ないことの確認に使う。
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevSlog, prevLog := slog.Default(), log.Writer()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	log.SetOutput(&buf)
	t.Cleanup(func() {
		slog.SetDefault(prevSlog)
		log.SetOutput(prevLog)
	})
	return &buf
}

func TestIdentityIsolation(t *testing.T) {
	ctx := context.Background()
	logs := captureLogs(t)
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

	const syntheticPhone = "000-0000-0000"
	const syntheticIdentity = `{"legal_name":"合成テスト氏名","address":"合成テスト住所","phone":"` + syntheticPhone + `"}`
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
	// 電話番号は (2) の身元登録で必須。形式が合わない値・欠けた登録・未知の項目は受け付けない。
	request("PUT", save, coachToken, `{"legal_name":"test","address":"test"}`, 422)
	request("PUT", save, coachToken, `{"legal_name":"test","address":"test","phone":""}`, 422)
	request("PUT", save, coachToken, `{"legal_name":"test","address":"test","phone":"not-a-phone"}`, 422)
	request("PUT", save, coachToken, `{"legal_name":"test","address":"test","phone":"0000000000","email":"x"}`, 400)
	request("PUT", save, coachToken, syntheticIdentity, 204)

	var sealedPhone []byte
	if err := vault.QueryRow(ctx, "SELECT phone_enc FROM identity_vault.identities WHERE coach_id=$1", coach).Scan(&sealedPhone); err != nil || sealedPhone == nil {
		t.Fatal("phone was not stored in the vault")
	}
	if bytes.Contains(sealedPhone, []byte(syntheticPhone)) {
		t.Fatal("plaintext phone stored in private identity table")
	}
	if _, err := svc.open(coach, "address", sealedPhone); err == nil {
		t.Fatal("phone ciphertext was accepted as another field")
	}
	if _, err := svc.open(uuid.New(), "phone", sealedPhone); err == nil {
		t.Fatal("phone ciphertext substitution was accepted")
	}

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
	if err := json.Unmarshal(private.Body.Bytes(), &own); err != nil || own.Revision == uuid.Nil || !strings.Contains(private.Body.String(), "合成テスト住所") ||
		!strings.Contains(private.Body.String(), syntheticPhone) {
		t.Fatal("owner did not get registered identity")
	}
	// 管理閲覧：値は失敗時にも出さず、一致したかどうかだけを判定する。
	admin := request("POST", adminView, staffToken, `{"purpose":"開示請求の合成試験"}`, 200)
	var viewed struct {
		Identity identityFields `json:"identity"`
	}
	if err := json.Unmarshal(admin.Body.Bytes(), &viewed); err != nil || viewed.Identity.Phone != syntheticPhone ||
		viewed.Identity.Address != "合成テスト住所" || !strings.Contains(admin.Body.String(), staff.String()) ||
		!strings.Contains(admin.Body.String(), "開示請求の合成試験") {
		t.Fatal("administrator disclosure or audit record did not match")
	}

	consent := `{"confirm":true,"identity_revision":"` + own.Revision.String() + `"}`
	// 表記要でも、本人が表示内容を確認するまでは電話番号を含め何も出さない。
	request("GET", public, machine, "", 404)
	if plans := request("GET", "/internal/disclosures?ids="+plan.String(), machine, "", 200); strings.Contains(plans.Body.String(), plan.String()) {
		t.Fatal("unconfirmed plan was listed as approved")
	}
	// 電話番号は確認時に受け取らない（登録した版をそのまま公開する）。
	request("PUT", publish, coachToken, `{"confirm":true,"phone":"`+syntheticPhone+`","identity_revision":"`+own.Revision.String()+`"}`, 400)
	request("PUT", publish, coachToken, `{"confirm":false,"identity_revision":"`+own.Revision.String()+`"}`, 422)
	request("GET", public, machine, "", 404)
	request("PUT", publish, coachToken, consent, 204)
	if legacyPhoneColumn(t, vault) {
		t.Fatal("plaintext phone column still exists on the disclosure grant")
	}
	if plans := request("GET", "/internal/disclosures?ids="+plan.String(), machine, "", 200); !strings.Contains(plans.Body.String(), plan.String()) {
		t.Fatal("approved plan was hidden from the plan list")
	}
	res := request("GET", public, machine, "", 200)
	if !strings.Contains(res.Body.String(), "合成テスト住所") || !strings.Contains(res.Body.String(), syntheticPhone) || res.Header().Get("Cache-Control") != "no-store" {
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
	// 登録済みの電話番号がない（#440 の移行前の）行は、表記要の公開を認めない。
	execTest(t, vault, "UPDATE identity_vault.identities SET phone_enc=NULL WHERE coach_id=$1", coach)
	request("GET", public, machine, "", 404)
	if plans := request("GET", "/internal/disclosures?ids="+plan.String(), machine, "", 200); strings.Contains(plans.Body.String(), plan.String()) {
		t.Fatal("plan without a registered phone was listed as approved")
	}
	request("PUT", publish, coachToken, consent, 409)
	request("PUT", save, coachToken, syntheticIdentity, 204)
	request("PUT", publish, coachToken, consent, 409)
	refreshed := request("GET", viewCoach, coachToken, "", 200)
	if err := json.Unmarshal(refreshed.Body.Bytes(), &own); err != nil {
		t.Fatal(err)
	}
	consent = `{"confirm":true,"identity_revision":"` + own.Revision.String() + `"}`
	request("PUT", publish, coachToken, consent, 204)
	request("GET", public, machine, "", 200)

	// 表記不要のコーチは、登録済みでも電話番号（ほかの身元情報も）を公開経路に出さない。
	request("PUT", "/v1/coach/exemption", coachToken, `{"exempt":true,"checklist_version":1,"checked":[true,true,true]}`, 204)
	request("GET", public, machine, "", 404)
	request("PUT", publish, coachToken, consent, 204)
	if out := request("GET", public, machine, "", 200); strings.TrimSpace(out.Body.String()) != `{"required":false}` {
		t.Fatal("exempt plan exposed fields")
	}
	if plans := request("GET", "/internal/disclosures?ids="+plan.String(), machine, "", 200); !strings.Contains(plans.Body.String(), plan.String()) ||
		strings.Contains(plans.Body.String(), syntheticPhone) {
		t.Fatal("exempt plan list did not match")
	}
	request("PUT", save, coachToken, syntheticIdentity, 204)
	request("GET", public, machine, "", 404)
	request("PUT", publish, coachToken, consent, 409)

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

	for _, plain := range []string{syntheticPhone, "合成テスト住所", "合成テスト氏名"} {
		if strings.Contains(logs.String(), plain) {
			t.Fatal("identity plaintext appeared in logs")
		}
	}
}

// H1 で schema.sql だけを適用した金庫に、#440 の移行を追加だけで足せること。
func TestIdentityPhoneMigration(t *testing.T) {
	ctx := context.Background()
	vault := testVault(t)
	execTest(t, vault, readVaultFile(t, "schema.sql"))
	coach := uuid.New()
	execTest(t, vault, "INSERT INTO identity_vault.identities (coach_id,key_id,legal_name_enc,address_enc) VALUES ($1,'v1','\\x00','\\x00')", coach)
	migration := readVaultFile(t, filepath.Join("migrations", "002_identity_phone.sql"))
	execTest(t, vault, migration)
	execTest(t, vault, migration)
	var n int
	var phone []byte
	if err := vault.QueryRow(ctx, "SELECT count(*) FROM identity_vault.identities").Scan(&n); err != nil || n != 1 {
		t.Fatal("existing identity rows changed")
	}
	if err := vault.QueryRow(ctx, "SELECT phone_enc FROM identity_vault.identities WHERE coach_id=$1", coach).Scan(&phone); err != nil || phone != nil {
		t.Fatal("existing identity did not keep an empty phone")
	}
	var rls bool
	if err := vault.QueryRow(ctx, "SELECT relrowsecurity FROM pg_class WHERE oid='identity_vault.identities'::regclass").Scan(&rls); err != nil || !rls {
		t.Fatal("row level security was not kept")
	}
}

// #514：旧方式の平文列 disclosures.phone を消す移行を、平文列が残る金庫（schema.sql＋002〜004 を当てた
// 本番相当の形に、旧列を足したもの）に当てても、許可の行は残り、平文列だけが消えること。2回流しても同じ。
func TestDropDisclosurePhoneMigration(t *testing.T) {
	ctx := context.Background()
	vault := testVault(t)
	execTest(t, vault, readVaultFile(t, "schema.sql"))
	for _, f := range []string{"002_identity_phone.sql", "003_synthetic_provision.sql", "004_app_role_policies.sql"} {
		execTest(t, vault, readVaultFile(t, filepath.Join("migrations", f)))
	}
	execTest(t, vault, "ALTER TABLE identity_vault.disclosures ADD COLUMN phone text CHECK (phone IS NULL OR length(phone) BETWEEN 10 AND 20)")
	coach, plan := uuid.New(), uuid.New()
	execTest(t, vault, "INSERT INTO identity_vault.identities (coach_id,key_id,legal_name_enc,address_enc) VALUES ($1,'v1','\\x00','\\x00')", coach)
	execTest(t, vault, "INSERT INTO identity_vault.disclosures (plan_id,coach_id,plan_epoch,coach_epoch,phone) VALUES ($1,$2,$3,$4,'0000000000')", plan, coach, uuid.New(), uuid.New())
	if !legacyPhoneColumn(t, vault) {
		t.Fatal("test setup did not create the legacy column")
	}
	migration := readVaultFile(t, filepath.Join("migrations", "005_drop_disclosure_phone.sql"))
	execTest(t, vault, migration)
	execTest(t, vault, migration)
	if legacyPhoneColumn(t, vault) {
		t.Fatal("plaintext phone column was not dropped")
	}
	var n int
	if err := vault.QueryRow(ctx, "SELECT count(*) FROM identity_vault.disclosures WHERE plan_id=$1 AND coach_id=$2", plan, coach).Scan(&n); err != nil || n != 1 {
		t.Fatal("disclosure grant rows changed")
	}
	var rls bool
	if err := vault.QueryRow(ctx, "SELECT relrowsecurity FROM pg_class WHERE oid='identity_vault.disclosures'::regclass").Scan(&rls); err != nil || !rls {
		t.Fatal("row level security was not kept")
	}
}

func legacyPhoneColumn(t *testing.T, pool *pgxpool.Pool) bool {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_schema='identity_vault' AND table_name='disclosures' AND column_name='phone')`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	return exists
}

func execTest(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

// testDB は、ローカルの試験用サーバーに使い捨ての DB を作る。
func testDB(t *testing.T, prefix string) (string, *pgxpool.Pool) {
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

func testVault(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_, vault := testDB(t, "vault_test_")
	return vault
}

func testDatabases(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()
	mainDSN, main := testDB(t, "main_test_")
	if err := migrate.Up(mainDSN); err != nil {
		t.Fatal(err)
	}
	vault := testVault(t)
	execTest(t, vault, readVaultFile(t, "schema.sql"))
	migrations, err := filepath.Glob(filepath.Join(vaultDir, "migrations", "*.sql"))
	if err != nil || len(migrations) == 0 {
		t.Fatal("vault migrations not found")
	}
	slices.Sort(migrations)
	for _, file := range migrations {
		rel, err := filepath.Rel(vaultDir, file)
		if err != nil {
			t.Fatal(err)
		}
		execTest(t, vault, readVaultFile(t, rel))
	}
	return main, vault
}

var vaultDir = filepath.Join("..", "..", "..", "..", "deploy", "identity-vault")

func readVaultFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(vaultDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
