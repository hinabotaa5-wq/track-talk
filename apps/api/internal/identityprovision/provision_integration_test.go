package identityprovision_test

// 隔離環境の統合テスト（ops #63）。公式の Supabase Auth（GoTrue）と Postgres 2台（本体・金庫）を
// 外に出られないネットワークで動かし、その中でこのテストを実行する。本番には接続しない。
// 環境変数が無ければ skip する（CI の通常の go test では動かない）。
//
//	PROVISION_TEST_MAIN_DSN   本体相当の Postgres（スーパーユーザー）
//	PROVISION_TEST_VAULT_DSN  金庫相当の Postgres（スーパーユーザー。GoTrue が auth スキーマを作った DB）
//	PROVISION_TEST_AUTH_URL   GoTrue の URL（/auth/v1 を付けない）
//	PROVISION_TEST_JWT_SECRET GoTrue の JWT 秘密（試験用に生成したもの）

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hinata/track-talk/apps/api/internal/auth"
	"github.com/hinata/track-talk/apps/api/internal/identityprovision"
	"github.com/hinata/track-talk/apps/api/internal/identityvault"
	appmigrate "github.com/hinata/track-talk/apps/api/internal/migrate"
)

const base = "ops.inbox@example.test"

type env struct {
	runner      *identityprovision.Runner
	vaultAdmin  *pgxpool.Pool
	mainAdmin   *pgxpool.Pool
	authURL     string // プロキシ経由（…/auth/v1）
	adminKey    string
	publicKey   string
	synthCoach  uuid.UUID
	realCoach   uuid.UUID
	pendCoach   uuid.UUID
	realSubject uuid.UUID
	adminID     uuid.UUID
}

func sign(t *testing.T, secret, role string) string {
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"role": role, "iss": "supabase", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func mustExec(t *testing.T, p *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := p.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%v: %s", err, strings.SplitN(sql, "\n", 2)[0])
	}
}

func readRepo(t *testing.T, rel string) string {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// adminCreate は試験の前提（実在相当のユーザー・管理者）を GoTrue の管理 API で作る。
func adminCreate(t *testing.T, authURL, key, email string) uuid.UUID {
	body, _ := json.Marshal(map[string]any{"email": email, "password": uuid.NewString(), "email_confirm": true})
	req, _ := http.NewRequest(http.MethodPost, authURL+"/admin/users", bytes.NewReader(body))
	req.Header.Set("apikey", key)
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		ID uuid.UUID `json:"id"`
	}
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || json.Unmarshal(raw, &out) != nil {
		t.Fatalf("admin create %d", res.StatusCode)
	}
	return out.ID
}

func setup(t *testing.T) *env {
	mainDSN, vaultDSN := os.Getenv("PROVISION_TEST_MAIN_DSN"), os.Getenv("PROVISION_TEST_VAULT_DSN")
	gotrue, secret := os.Getenv("PROVISION_TEST_AUTH_URL"), os.Getenv("PROVISION_TEST_JWT_SECRET")
	if mainDSN == "" || vaultDSN == "" || gotrue == "" || secret == "" {
		t.Skip("isolated provisioning environment not configured")
	}
	ctx := context.Background()
	e := &env{adminKey: sign(t, secret, "service_role"), publicKey: sign(t, secret, "anon")}

	// /auth/v1 を剥がして GoTrue に渡す中継（本番の URL 形と同じにするため）。
	target, _ := url.Parse(gotrue)
	proxy := httputil.NewSingleHostReverseProxy(target)
	front := httptest.NewServer(http.StripPrefix("/auth/v1", proxy))
	t.Cleanup(front.Close)
	e.authURL = front.URL + "/auth/v1"

	// 本体相当：新しい DB に最新まで migration を当て、本体読み取りロールを本番の H4 と同じ形で用意する。
	mainAdmin, err := pgxpool.New(ctx, mainDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mainAdmin.Close)
	name := "prov_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	mustExec(t, mainAdmin, "CREATE DATABASE "+name)
	t.Cleanup(func() { _, _ = mainAdmin.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)") })
	u, _ := url.Parse(mainDSN)
	u.Path = "/" + name
	mainIsolated := u.String()
	if err := appmigrate.Up(mainIsolated); err != nil {
		t.Fatal(err)
	}
	e.mainAdmin, err = pgxpool.New(ctx, mainIsolated)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.mainAdmin.Close)
	mustExec(t, e.mainAdmin, `CREATE SCHEMA IF NOT EXISTS auth; CREATE TABLE IF NOT EXISTS auth.users (id uuid PRIMARY KEY, email text, deleted_at timestamptz)`)
	reader := "reader_" + name
	mustExec(t, e.mainAdmin, "CREATE ROLE "+reader+" LOGIN PASSWORD 'readerpw'")
	t.Cleanup(func() {
		_, _ = mainAdmin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		_, _ = mainAdmin.Exec(context.Background(), "DROP ROLE IF EXISTS "+reader)
	})
	mustExec(t, e.mainAdmin, "GRANT USAGE ON SCHEMA identity_boundary TO "+reader+"; GRANT SELECT ON ALL TABLES IN SCHEMA identity_boundary TO "+reader+
		"; GRANT EXECUTE ON FUNCTION identity_boundary.mark_retired(uuid) TO "+reader)
	// 000154 は mark_retired の付与を見て EXECUTE を付けるため、付与の後にもう一度当てる（本番の順序と同じ：H4 が先）。
	mustExec(t, e.mainAdmin, readRepo(t, "apps/api/migrations/000154_identity_coach_email_match.down.sql"))
	mustExec(t, e.mainAdmin, readRepo(t, "apps/api/migrations/000154_identity_coach_email_match.up.sql"))
	e.synthCoach, e.realCoach, e.pendCoach = uuid.New(), uuid.New(), uuid.New()
	for i, c := range []struct {
		id            uuid.UUID
		status, email string
	}{
		{e.synthCoach, "approved", "ops.inbox+synthetic-aaa@example.test"},
		{e.realCoach, "approved", "real.coach@example.test"},
		{e.pendCoach, "pending", "ops.inbox+synthetic-ppp@example.test"},
	} {
		mustExec(t, e.mainAdmin, `INSERT INTO profiles (id, nickname) VALUES ($1, $2)`, c.id, "prov"+string(rune('a'+i)))
		mustExec(t, e.mainAdmin, `INSERT INTO coach_profiles (profile_id, tier, events, best_record, achievements, years, status, reviewed_at)
			VALUES ($1, 'coach', ARRAY['100m'], '-', '-', 1, $2, CASE WHEN $2 = 'pending' THEN NULL ELSE now() END)`, c.id, c.status)
		mustExec(t, e.mainAdmin, `INSERT INTO auth.users (id, email) VALUES ($1, $2)`, c.id, c.email)
	}
	ru, _ := url.Parse(mainIsolated)
	ru.User = url.UserPassword(reader, "readerpw")
	mainReader, err := pgxpool.New(ctx, ru.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mainReader.Close)

	// 金庫相当：schema.sql と migrations を所有者で当て、アプリ用ロール・輸出用ロールを本番の H1〜H5 と同じ形で用意する。
	e.vaultAdmin, err = pgxpool.New(ctx, vaultDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.vaultAdmin.Close)
	mustExec(t, e.vaultAdmin, "DROP SCHEMA IF EXISTS identity_vault CASCADE")
	mustExec(t, e.vaultAdmin, "DELETE FROM auth.users WHERE email LIKE 'ops.inbox+synthetic-%'")
	mustExec(t, e.vaultAdmin, `DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='identity_app') THEN CREATE ROLE identity_app LOGIN PASSWORD 'apppw'; END IF;
		-- 本番と同じく、アプリ用ロールは RLS を素通しできない（2026-11-11 の本番の確認で app_roles_bypass_rls=0）。
		ALTER ROLE identity_app NOBYPASSRLS;
		IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='identity_export') THEN CREATE ROLE identity_export LOGIN BYPASSRLS NOINHERIT; END IF;
		IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='service_role') THEN CREATE ROLE service_role NOLOGIN; END IF;
	END $$`)
	mustExec(t, e.vaultAdmin, readRepo(t, "deploy/identity-vault/schema.sql"))
	mustExec(t, e.vaultAdmin, `GRANT USAGE ON SCHEMA identity_vault TO identity_app, identity_export;
		GRANT SELECT ON identity_vault.members TO identity_app;
		GRANT SELECT, INSERT, UPDATE, DELETE ON identity_vault.identities, identity_vault.commerce, identity_vault.disclosures TO identity_app;
		GRANT SELECT, INSERT ON identity_vault.access_log TO identity_app;
		GRANT SELECT ON ALL TABLES IN SCHEMA identity_vault TO identity_export`)
	for _, f := range []string{"002_identity_phone.sql", "003_synthetic_provision.sql", "004_app_role_policies.sql"} {
		mustExec(t, e.vaultAdmin, readRepo(t, "deploy/identity-vault/migrations/"+f))
	}
	// 管理者（管理者行）と、合成でない実在相当のコーチの割り当て。
	e.adminID = adminCreate(t, e.authURL, e.adminKey, "admin."+name+"@example.test")
	e.realSubject = adminCreate(t, e.authURL, e.adminKey, "real."+name+"@example.test")
	mustExec(t, e.vaultAdmin, `INSERT INTO identity_vault.members (subject, coach_id, is_admin) VALUES ($1, NULL, true), ($2, $3, false)`, e.adminID, e.realSubject, e.realCoach)
	// 管理者は二段階認証を登録済み（本番の前提と同じ）。
	mustExec(t, e.vaultAdmin, `INSERT INTO auth.mfa_factors (id, user_id, friendly_name, factor_type, status, created_at, updated_at, secret)
		VALUES (gen_random_uuid(), $1, 'admin', 'totp', 'verified', now(), now(), 'x')`, e.adminID)
	mustExec(t, e.vaultAdmin, `INSERT INTO identity_vault.identities (coach_id, key_id, legal_name_enc, address_enc) VALUES ($1, 'v1', '\x00', '\x00')`, e.realCoach)
	t.Cleanup(func() {
		_, _ = e.vaultAdmin.Exec(context.Background(), "DELETE FROM identity_vault.members WHERE subject = ANY($1)", []uuid.UUID{e.adminID, e.realSubject})
	})

	vu, _ := url.Parse(vaultDSN)
	vu.User = url.UserPassword("identity_app", "apppw")
	vaultApp, err := pgxpool.New(ctx, vu.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vaultApp.Close)
	authClient, err := identityprovision.NewAuth(e.authURL, e.adminKey, e.publicKey)
	if err != nil {
		t.Fatal(err)
	}
	e.runner = &identityprovision.Runner{Main: mainReader, Vault: vaultApp, Auth: authClient, Base: base}

	// 通し確認の相手：本物の身元専用サービス（同じ金庫・本体の接続、同じ GoTrue でセッション確認）。
	verifier, err := auth.NewVerifier(secret, "", "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	svc, err := identityvault.New(mainReader, vaultApp, verifier, auth.Admin{BaseURL: front.URL, APIKey: e.publicKey},
		"AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=", strings.Repeat("r", 32), strings.Repeat("c", 32), strings.Repeat("e", 32), "https://web.example.test")
	if err != nil {
		t.Fatal(err)
	}
	identitySrv := httptest.NewServer(svc.Handler())
	t.Cleanup(identitySrv.Close)
	e.runner.IdentityURL = identitySrv.URL
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"coach":{"nickname":"合成","events":["100m"]}}`))
	}))
	t.Cleanup(public.Close)
	e.runner.PublicURL = public.URL
	return e
}

func expect(t *testing.T, r identityprovision.Result, kv map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(r)
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	for k, want := range kv {
		if w, ok := want.(int); ok {
			want = float64(w)
		}
		if got[k] != want {
			t.Errorf("%s = %v, want %v (result %s)", k, got[k], want, raw)
		}
	}
	// 出力にメール・subject・秘密が出ないこと。
	for _, leak := range []string{"@", "synthetic-aaa", "service_role", "password", "secret", "totp"} {
		if bytes.Contains(raw, []byte(leak)) {
			t.Errorf("output contains %q: %s", leak, raw)
		}
	}
}

func TestProvisionVerifyWalkthroughCleanup(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	r := e.runner

	// コーチ ID は合成メールから本体の実物で引く（未承認は見つからない）。
	lookup := r.Lookup(ctx, "aaa")
	if lookup["coach_id"] != e.synthCoach.String() || lookup["found"] != true {
		t.Fatalf("lookup %v", lookup)
	}
	if got := r.Lookup(ctx, "ppp"); got["found"] != false || got["coach_id"] != nil {
		t.Fatalf("lookup pending %v", got)
	}
	expect(t, r.Verify(ctx, e.synthCoach, "aaa"), map[string]any{"result": "mismatch", "main_guard": true, "auth_user_exists": false, "member_rows": 0})
	expect(t, r.Provision(ctx, e.synthCoach, "aaa"), map[string]any{"result": "ok", "created_user": true, "binding": "created",
		"member_rows": 1, "subject_matches": true, "subject_synthetic": true, "admin_rows": 0})
	// 冪等：2回目は作らない。
	expect(t, r.Provision(ctx, e.synthCoach, "aaa"), map[string]any{"result": "ok", "created_user": false, "binding": "exists"})
	expect(t, r.Verify(ctx, e.synthCoach, "aaa"), map[string]any{"result": "match", "member_rows": 1, "subject_matches": true})

	// 通し確認：TOTP の登録まで実行体の中で済ませ、aal2 で身元画面を読み、架空の身元を登録する。
	expect(t, r.Walkthrough(ctx, e.synthCoach, "aaa"), map[string]any{"result": "ok", "aal2_session": true, "coach_view_status": 200,
		"identity_save_status": 204, "identity_read_back_matches": true, "public_api_exposes_identity": false,
		"admin_member_rows": 0 + 1, "identity_rows": 1})
	// 2回目も通る（パスワード・TOTP を付け直す）。
	expect(t, r.Walkthrough(ctx, e.synthCoach, "aaa"), map[string]any{"result": "ok"})
	// 退会の前は終了経路の確認が通らない。
	expect(t, r.EndCheck(ctx, e.synthCoach, "aaa"), map[string]any{"result": "failed", "approved_on_main": true, "retired_on_main": false})
	// 本体で退会（画面操作 /coach/me/withdraw と同じ状態の変化）→ 承認済みでなくなり、保持に載り、身元画面は 403。
	mustExec(t, e.mainAdmin, "UPDATE coach_profiles SET status = 'withdrawn' WHERE profile_id = $1", e.synthCoach)
	expect(t, r.EndCheck(ctx, e.synthCoach, "aaa"), map[string]any{"result": "ok", "approved_on_main": false, "retired_on_main": true,
		"coach_view_status_after_end": 403, "identity_rows": 1})
	// 以降の確認のため、承認済みに戻す（試験の都合）。
	mustExec(t, e.mainAdmin, "UPDATE coach_profiles SET status = 'approved' WHERE profile_id = $1", e.synthCoach)

	// subject のずれ：members 行を別の合成ユーザーに手で付け替えると、verify が不一致として検出する。
	other := adminCreate(t, e.authURL, e.adminKey, "ops.inbox+synthetic-zzz@example.test")
	mustExec(t, e.vaultAdmin, "UPDATE identity_vault.members SET subject = $1 WHERE coach_id = $2", other, e.synthCoach)
	expect(t, r.Verify(ctx, e.synthCoach, "aaa"), map[string]any{"result": "mismatch", "member_rows": 1, "subject_matches": false, "subject_synthetic": true})
	// ずれた状態の provision は自動では付け替えない。
	expect(t, r.Provision(ctx, e.synthCoach, "aaa"), map[string]any{"result": "failed", "error": "synthetic_coach_bound_elsewhere"})

	// cleanup：ずれた割り当て・身元行・2人の合成ユーザーを消す。管理者・実在相当には触れない。
	expect(t, r.Cleanup(ctx, e.synthCoach, "aaa"), map[string]any{"result": "ok", "deleted_member_rows": 1, "deleted_identity_rows": 1,
		"deleted_auth_users": 2, "remaining_member_rows": 0, "remaining_identity_rows": 0})
	var admins, realRows, realIdentity int
	if err := e.vaultAdmin.QueryRow(ctx, `SELECT (SELECT count(*) FROM identity_vault.members WHERE is_admin),
		(SELECT count(*) FROM identity_vault.members WHERE coach_id=$1), (SELECT count(*) FROM identity_vault.identities WHERE coach_id=$1)`, e.realCoach).
		Scan(&admins, &realRows, &realIdentity); err != nil {
		t.Fatal(err)
	}
	if admins != 1 || realRows != 1 || realIdentity != 1 {
		t.Fatalf("admin/real rows changed: %d %d %d", admins, realRows, realIdentity)
	}
	var authUsers int
	if err := e.vaultAdmin.QueryRow(ctx, "SELECT count(*) FROM auth.users WHERE id = ANY($1)", []uuid.UUID{e.adminID, e.realSubject}).Scan(&authUsers); err != nil || authUsers != 2 {
		t.Fatalf("admin/real auth users: %d %v", authUsers, err)
	}
	// 片付け後に作り直せる（フェーズ3の cleanup→provision と同じ）。
	expect(t, r.Provision(ctx, e.synthCoach, "aaa"), map[string]any{"result": "ok", "created_user": true, "binding": "created"})
	expect(t, r.Cleanup(ctx, e.synthCoach, "aaa"), map[string]any{"result": "ok", "deleted_member_rows": 1, "deleted_auth_users": 1})
}

func TestProvisionGuards(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	r := e.runner
	// 合成パターン外：タグの形が違う・元のアドレスが設定されていない。
	expect(t, r.Provision(ctx, e.synthCoach, "AAA!"), map[string]any{"result": "failed", "error": "synthetic_email_rejected"})
	bad := *r
	bad.Base = ""
	expect(t, bad.Provision(ctx, e.synthCoach, "aaa"), map[string]any{"result": "failed", "error": "synthetic_email_rejected"})
	// コーチ ID とメールの不一致（実在相当のコーチ ID に合成メールを結び付けようとする）。
	expect(t, r.Provision(ctx, e.realCoach, "aaa"), map[string]any{"result": "failed", "error": "coach_not_approved_or_email_mismatch"})
	// 未承認のコーチ。
	expect(t, r.Provision(ctx, e.pendCoach, "ppp"), map[string]any{"result": "failed", "error": "coach_not_approved_or_email_mismatch"})
	// 実在相当のコーチの割り当ては cleanup でも消さない（subject が合成でない）。
	expect(t, r.Cleanup(ctx, e.realCoach, "aaa"), map[string]any{"result": "failed", "error": "binding_not_synthetic"})
	var rows int
	if err := e.vaultAdmin.QueryRow(ctx, "SELECT count(*) FROM identity_vault.members WHERE coach_id=$1", e.realCoach).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("real binding removed: %d %v", rows, err)
	}
	// 何も作られていない（金庫 Auth の合成ユーザーも、割り当ても）。
	expect(t, r.Verify(ctx, e.synthCoach, "aaa"), map[string]any{"auth_user_exists": false, "member_rows": 0})
}

// 金庫側の関数そのもののガード：アプリ用ロールからでも、合成でない subject や管理者行は作れない・消せない。
func TestVaultFunctionsRefuseRealAndAdmin(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	vu, _ := url.Parse(os.Getenv("PROVISION_TEST_VAULT_DSN"))
	vu.User = url.UserPassword("identity_app", "apppw")
	app, err := pgx.Connect(ctx, vu.String())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close(ctx)
	code := func(sql string, args ...any) string {
		_, err := app.Exec(ctx, sql, args...)
		if err == nil {
			return ""
		}
		msg := err.Error()
		for _, c := range []string{"synthetic_subject_required", "synthetic_subject_bound_elsewhere", "synthetic_coach_bound_elsewhere", "synthetic_invalid_input", "permission denied"} {
			if strings.Contains(msg, c) {
				return c
			}
		}
		return msg
	}
	// 合成でない subject（管理者・実在相当）を新しいコーチに結び付けられない。
	if got := code("SELECT identity_vault.synthetic_bind($1, $2)", e.adminID, uuid.New()); got != "synthetic_subject_required" {
		t.Errorf("bind admin subject: %q", got)
	}
	if got := code("SELECT identity_vault.synthetic_bind($1, $2)", e.realSubject, uuid.New()); got != "synthetic_subject_required" {
		t.Errorf("bind real subject: %q", got)
	}
	// 合成の subject でも、実在相当のコーチ（既に割り当てあり）には結び付けられない。
	synth := adminCreate(t, e.authURL, e.adminKey, "ops.inbox+synthetic-vvv@example.test")
	if got := code("SELECT identity_vault.synthetic_bind($1, $2)", synth, e.realCoach); got != "synthetic_coach_bound_elsewhere" {
		t.Errorf("bind to real coach: %q", got)
	}
	// 実在相当の割り当ては消せない。
	if got := code("SELECT * FROM identity_vault.synthetic_unbind($1)", e.realCoach); got != "synthetic_subject_required" {
		t.Errorf("unbind real: %q", got)
	}
	// is_admin=true の行を作る経路は無い（関数は常に false で作る）。作った行を確かめる。
	if got := code("SELECT identity_vault.synthetic_bind($1, $2)", synth, uuid.New()); got != "" {
		t.Fatalf("bind synthetic: %q", got)
	}
	var adminRows int
	if err := e.vaultAdmin.QueryRow(ctx, "SELECT count(*) FROM identity_vault.members WHERE subject=$1 AND is_admin", synth).Scan(&adminRows); err != nil || adminRows != 0 {
		t.Fatalf("admin row created: %d %v", adminRows, err)
	}
	// アプリ用ロールは members に直接書けない（既存の原則のまま）。
	if got := code("INSERT INTO identity_vault.members (subject, coach_id, is_admin) VALUES ($1, NULL, true)", uuid.New()); got != "permission denied" {
		t.Errorf("direct insert: %q", got)
	}
	// 輸出用ロールは関数を実行できない。
	var exportCan bool
	if err := e.vaultAdmin.QueryRow(ctx, "SELECT has_function_privilege('identity_export', 'identity_vault.synthetic_bind(uuid, uuid)', 'EXECUTE')").Scan(&exportCan); err != nil || exportCan {
		t.Fatalf("export role can execute: %v %v", exportCan, err)
	}
	mustExec(t, e.vaultAdmin, "DELETE FROM identity_vault.members WHERE subject=$1", synth)
}

// 旧 aaa（合成の標識が無い割り当て）の一回限りの削除：2877c3f1- の1件だけ。管理者・他の行には触れない。
func TestLegacyCleanup(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	r := e.runner
	expect(t, r.LegacyCleanup(ctx), map[string]any{"result": "ok", "matched_rows": 0, "deleted_auth_users": 0})
	legacyCoach := uuid.MustParse("2877c3f1-1111-4111-8111-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12])
	legacySubject := adminCreate(t, e.authURL, e.adminKey, "legacy."+strings.ReplaceAll(uuid.NewString(), "-", "")[:8]+"@example.test")
	mustExec(t, e.vaultAdmin, "INSERT INTO identity_vault.members (subject, coach_id, is_admin) VALUES ($1, $2, false)", legacySubject, legacyCoach)
	mustExec(t, e.vaultAdmin, `INSERT INTO identity_vault.identities (coach_id, key_id, legal_name_enc, address_enc) VALUES ($1, 'v1', '\x00', '\x00')`, legacyCoach)
	// 2件あれば止まる。
	extra := uuid.MustParse("2877c3f1-2222-4222-8222-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12])
	extraSubject := adminCreate(t, e.authURL, e.adminKey, "legacy2."+strings.ReplaceAll(uuid.NewString(), "-", "")[:8]+"@example.test")
	mustExec(t, e.vaultAdmin, "INSERT INTO identity_vault.members (subject, coach_id, is_admin) VALUES ($1, $2, false)", extraSubject, extra)
	expect(t, r.LegacyCleanup(ctx), map[string]any{"result": "failed", "error": "legacy_ambiguous"})
	mustExec(t, e.vaultAdmin, "DELETE FROM identity_vault.members WHERE coach_id = $1", extra)
	got := r.LegacyCleanup(ctx)
	if got["legacy_coach_id"] != legacyCoach.String() {
		t.Fatalf("legacy coach %v", got)
	}
	delete(got, "legacy_coach_id")
	expect(t, got, map[string]any{"result": "ok", "matched_rows": 1, "deleted_member_rows": 1, "deleted_identity_rows": 1, "deleted_auth_users": 1})
	var admins, realRows, legacyUsers int
	if err := e.vaultAdmin.QueryRow(ctx, `SELECT (SELECT count(*) FROM identity_vault.members WHERE is_admin),
		(SELECT count(*) FROM identity_vault.members WHERE coach_id=$1), (SELECT count(*) FROM auth.users WHERE id=$2)`, e.realCoach, legacySubject).
		Scan(&admins, &realRows, &legacyUsers); err != nil {
		t.Fatal(err)
	}
	if admins != 1 || realRows != 1 || legacyUsers != 0 {
		t.Fatalf("after legacy cleanup: admins %d real %d legacy users %d", admins, realRows, legacyUsers)
	}
	// 2回目は何もしない。
	expect(t, r.LegacyCleanup(ctx), map[string]any{"result": "ok", "matched_rows": 0})
}

// 本番で起きた 403 の再現：RLS の方針（004）が無いと、aal2 のセッションでも身元画面に入れない。
// 004 を当て直すと通る。
func TestWalkthroughNeedsAppPolicies(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	r := e.runner
	expect(t, r.Provision(ctx, e.synthCoach, "aaa"), map[string]any{"result": "ok"})
	for _, tbl := range []string{"members", "identities", "commerce", "disclosures", "access_log"} {
		mustExec(t, e.vaultAdmin, "DROP POLICY IF EXISTS identity_service_app ON identity_vault."+tbl)
	}
	expect(t, r.Walkthrough(ctx, e.synthCoach, "aaa"), map[string]any{"result": "failed", "aal2_session": true, "coach_view_status": 403, "identity_save_status": 403})
	mustExec(t, e.vaultAdmin, readRepo(t, "deploy/identity-vault/migrations/004_app_role_policies.sql"))
	expect(t, r.Walkthrough(ctx, e.synthCoach, "aaa"), map[string]any{"result": "ok", "coach_view_status": 200, "identity_save_status": 204})
	// 方針はアプリ用ロールだけ。輸出用ロールや PUBLIC には付かない。
	var other int
	if err := e.vaultAdmin.QueryRow(ctx, `SELECT count(*) FROM pg_policies WHERE schemaname='identity_vault'
		AND roles && ARRAY['public','anon','authenticated','service_role','identity_export']::name[]`).Scan(&other); err != nil || other != 0 {
		t.Fatalf("policies for restricted roles: %d %v", other, err)
	}
	expect(t, r.Cleanup(ctx, e.synthCoach, "aaa"), map[string]any{"result": "ok"})
}
