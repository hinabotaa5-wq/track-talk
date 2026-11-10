// Command dbmigrate は、本番 DB に未適用の migration を当てる Cloud Run Job の本体（ops #63）。
//
// 公開 API は DATABASE_MIGRATIONS_EXTERNAL=true で起動時に migration を流さないため、
// migration はこのジョブだけが当てる。接続文字列は Job の秘密参照（環境変数 DATABASE_URL）からだけ受け取り、
// 起動する人（エージェント）の手元・引数・ログには渡らない。出力は1行の JSON で、版数・dirty・件数・真偽だけ。
// 接続先・SQL・エラーの原文は出さない（原文は接続情報や文の断片を含みうるため）。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hinata/track-talk/apps/api/internal/migrate"
)

type output struct {
	Event  string `json:"event"`
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
	migrate.Report
	Mode    string         `json:"mode"`
	Pending *int           `json:"pending,omitempty"`
	Checks  map[string]any `json:"checks,omitempty"`
}

// 引数なし（または apply）で適用、check で読み取りだけ（版・dirty・未適用の件数）。
func main() {
	mode := "apply"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	os.Exit(run(mode, os.Getenv("DATABASE_URL"), os.Stdout))
}

func run(mode, databaseURL string, w io.Writer) int {
	out := output{Event: "db_migrate", Result: "failed", Mode: mode}
	defer func() { _ = json.NewEncoder(w).Encode(out) }()
	if mode != "apply" && mode != "check" {
		out.Error = "unknown_mode"
		return 2
	}
	if err := checkURL(databaseURL); err != nil {
		out.Error = err.Error()
		return 2
	}
	if mode == "check" {
		report, pending, err := migrate.Status(databaseURL)
		out.Report, out.Pending = report, &pending
		if err != nil {
			out.Error = "status_failed"
			return 1
		}
		out.Result = "ok"
		return 0
	}
	report, err := migrate.Apply(databaseURL)
	out.Report = report
	switch {
	case errors.Is(err, migrate.ErrDirty):
		out.Error = "dirty_before_apply"
		return 3
	case errors.Is(err, migrate.ErrAhead):
		out.Error = "database_ahead_of_image"
		return 3
	case err != nil:
		out.Error = "migration_failed"
		return 1
	}
	if report.AfterDirty || report.AfterVersion != report.Expected {
		out.Error = "unexpected_version_after_apply"
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	checks, ok, err := postChecks(ctx, databaseURL, report.AfterVersion)
	out.Checks = checks
	if err != nil {
		out.Error = "post_check_failed"
		return 1
	}
	if !ok {
		out.Error = "post_check_mismatch"
		return 1
	}
	out.Result = "ok"
	return 0
}

// checkURL は、接続文字列の形と暗号化の有無だけを確かめる。値は返さない。
func checkURL(raw string) error {
	if raw == "" {
		return errors.New("database_url_missing")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" {
		return errors.New("invalid_database_configuration")
	}
	switch u.Hostname() {
	case "127.0.0.1", "localhost", "::1", "host.docker.internal":
		return nil
	}
	switch u.Query().Get("sslmode") {
	case "require", "verify-ca", "verify-full":
		return nil
	}
	return errors.New("encrypted_database_connection_required")
}

// postChecks は、適用後の状態を読み取り専用のトランザクションで確かめる。版ごとの確認を足すときはここに足す。
func postChecks(ctx context.Context, databaseURL string, version uint) (map[string]any, bool, error) {
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return nil, false, err
	}
	defer conn.Close(ctx)
	checks := map[string]any{}
	ok := true
	if version < 154 {
		return checks, ok, nil
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var definer, ownerMatches bool
	var config []string
	if err := tx.QueryRow(ctx, `
		SELECT f.prosecdef, COALESCE(f.proconfig, '{}'), f.proowner = r.proowner
		FROM pg_proc f, pg_proc r
		WHERE f.oid = 'identity_boundary.coach_matches_email(uuid, text)'::regprocedure
		  AND r.oid = 'identity_boundary.mark_retired(uuid)'::regprocedure`).Scan(&definer, &config, &ownerMatches); err != nil {
		return checks, false, err
	}
	searchPathFixed := len(config) == 1 && config[0] == "search_path=pg_catalog"
	checks["coach_matches_email_definer"] = definer
	checks["coach_matches_email_search_path_fixed"] = searchPathFixed
	checks["coach_matches_email_owner_matches_boundary"] = ownerMatches
	var apiRoles int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM pg_roles
		WHERE rolname IN ('anon', 'authenticated', 'service_role', 'track_talk_api')
		  AND has_function_privilege(oid, 'identity_boundary.coach_matches_email(uuid, text)', 'EXECUTE')`).Scan(&apiRoles); err != nil {
		return checks, false, err
	}
	checks["api_roles_can_execute"] = apiRoles
	var publicExec bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM pg_proc p CROSS JOIN LATERAL aclexplode(p.proacl) a
		               WHERE p.oid = 'identity_boundary.coach_matches_email(uuid, text)'::regprocedure AND a.grantee = 0)`).Scan(&publicExec); err != nil {
		return checks, false, err
	}
	checks["public_can_execute"] = publicExec
	rows, err := tx.Query(ctx, `
		SELECT g.rolname
		FROM pg_proc p CROSS JOIN LATERAL aclexplode(p.proacl) a JOIN pg_roles g ON g.oid = a.grantee
		WHERE p.oid = 'identity_boundary.mark_retired(uuid)'::regprocedure
		  AND a.privilege_type = 'EXECUTE' AND a.grantee <> p.proowner AND g.rolcanlogin AND NOT g.rolsuper
		  AND g.rolname NOT IN ('anon', 'authenticated', 'service_role', 'track_talk_api')`)
	if err != nil {
		return checks, false, err
	}
	readers, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return checks, false, err
	}
	checks["boundary_reader_roles"] = len(readers)
	canExecute, callOK, setRoleDenied := 0, 0, 0
	for i, role := range readers {
		var can bool
		if err := tx.QueryRow(ctx, `SELECT has_function_privilege($1, 'identity_boundary.coach_matches_email(uuid, text)', 'EXECUTE')`, role).Scan(&can); err != nil {
			return checks, false, err
		}
		if can {
			canExecute++
		}
		// 対象ロールとして実際に1回呼ぶ（存在しない ID なので結果は必ず false）。呼べたかどうかだけを数える。
		sp := fmt.Sprintf("reader_call_%d", i)
		if _, err := tx.Exec(ctx, "SAVEPOINT "+sp); err != nil {
			return checks, false, err
		}
		var matched bool
		_, setErr := tx.Exec(ctx, "SET LOCAL ROLE "+pgx.Identifier{role}.Sanitize())
		if setErr != nil {
			// 実行ロールがそのロールへ切り替えられない構成（PG16 以降の CREATEROLE の既定など）。
			// 実行権そのものは上の has_function_privilege で判定済みなので、ここは参考値として数えるだけ。
			setRoleDenied++
		} else if err := tx.QueryRow(ctx, `SELECT identity_boundary.coach_matches_email('00000000-0000-0000-0000-000000000000', 'nobody@invalid.example')`).Scan(&matched); err == nil && !matched {
			callOK++
		}
		if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+sp); err != nil {
			return checks, false, err
		}
		if _, err := tx.Exec(ctx, "RESET ROLE"); err != nil {
			return checks, false, err
		}
	}
	checks["boundary_readers_can_execute"] = canExecute
	checks["boundary_readers_call_ok"] = callOK
	checks["boundary_readers_set_role_denied"] = setRoleDenied
	// 実際の呼び出しは、切り替えられたロールでは必ず成功していること（切り替えられないロールは参考値）。
	ok = definer && searchPathFixed && ownerMatches && apiRoles == 0 && !publicExec &&
		len(readers) >= 1 && canExecute == len(readers) && callOK+setRoleDenied == len(readers)
	return checks, ok, nil
}
