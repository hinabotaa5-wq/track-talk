// Command identityprovision は、合成データ専用のプロビジョニングの Cloud Run Job
// （track-talk-identity-provision）の本体（ops #63）。
//
//	identityprovision provision   <本体コーチ ID> <タグ>
//	identityprovision verify      <本体コーチ ID> <タグ>
//	identityprovision cleanup     <本体コーチ ID> <タグ>
//	identityprovision walkthrough <本体コーチ ID> <タグ>
//	identityprovision endcheck    <本体コーチ ID> <タグ>   （退会の後、終了経路の確認）
//	identityprovision lookup      <タグ>                    （合成コーチの本体 ID を引く）
//	identityprovision legacy-cleanup                       （旧 aaa の1件だけ。運営者決定 2026-11-10）
//
// 合成メールはジョブの設定値 SYNTHETIC_EMAIL_BASE とタグから作る（起動の引数にメールは渡さない）。
// 資格情報はすべてジョブの秘密参照から環境変数で受け取り、出力は1行の JSON（操作名・成否・件数・真偽・時刻）だけ。
package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/identityprovision"
)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout))
}

func emit(w io.Writer, r identityprovision.Result) {
	r["event"] = "identity_synthetic"
	r["at"] = time.Now().UTC().Format(time.RFC3339)
	_ = json.NewEncoder(w).Encode(r)
}

func run(args []string, env func(string) string, w io.Writer) int {
	r := identityprovision.Result{}
	if len(args) == 0 {
		r["result"], r["error"] = "failed", "usage"
		emit(w, r)
		return 2
	}
	op := args[0]
	r["op"] = op
	var coach uuid.UUID
	var tag string
	switch op {
	case "provision", "verify", "cleanup", "walkthrough", "endcheck":
		if len(args) != 3 {
			r["result"], r["error"] = "failed", "usage"
			emit(w, r)
			return 2
		}
		var err error
		coach, err = uuid.Parse(args[1])
		if err != nil || coach == uuid.Nil {
			r["result"], r["error"] = "failed", "coach_id_invalid"
			emit(w, r)
			return 2
		}
		tag = args[2]
	case "lookup":
		if len(args) != 2 {
			r["result"], r["error"] = "failed", "usage"
			emit(w, r)
			return 2
		}
		tag = args[1]
	case "legacy-cleanup":
		if len(args) != 1 {
			r["result"], r["error"] = "failed", "usage"
			emit(w, r)
			return 2
		}
	default:
		r["result"], r["error"] = "failed", "unknown_op"
		emit(w, r)
		return 2
	}
	mainURL, vaultURL := env("IDENTITY_MAIN_READ_DATABASE_URL"), env("IDENTITY_DATABASE_URL")
	if mainURL == "" || vaultURL == "" || mainURL == vaultURL ||
		config.RequireLocalDatabase(mainURL) != nil || config.RequireLocalDatabase(vaultURL) != nil {
		r["result"], r["error"] = "failed", "database_configuration_invalid"
		emit(w, r)
		return 2
	}
	authURL := strings.TrimRight(env("IDENTITY_AUTH_URL"), "/")
	if path := env("IDENTITY_AUTH_PATH"); path != "" {
		authURL += path
	} else {
		authURL += "/auth/v1"
	}
	authClient, err := identityprovision.NewAuth(authURL, env("IDENTITY_AUTH_ADMIN_KEY"), env("IDENTITY_AUTH_PUBLIC_KEY"))
	if err != nil {
		r["result"], r["error"] = "failed", err.Error()
		emit(w, r)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	mainDB, err := db.NewPool(ctx, mainURL, "track-talk-identity-read")
	if err != nil {
		r["result"], r["error"] = "failed", "main_database_unavailable"
		emit(w, r)
		return 1
	}
	defer mainDB.Close()
	vaultDB, err := db.NewPool(ctx, vaultURL, "track-talk-identity")
	if err != nil {
		r["result"], r["error"] = "failed", "identity_database_unavailable"
		emit(w, r)
		return 1
	}
	defer vaultDB.Close()
	rn := &identityprovision.Runner{Main: mainDB, Vault: vaultDB, Auth: authClient, Base: env("SYNTHETIC_EMAIL_BASE"),
		IdentityURL: env("IDENTITY_SERVICE_URL"), PublicURL: env("PUBLIC_API_URL")}
	switch op {
	case "provision":
		r = rn.Provision(ctx, coach, tag)
	case "verify":
		r = rn.Verify(ctx, coach, tag)
	case "cleanup":
		r = rn.Cleanup(ctx, coach, tag)
	case "walkthrough":
		r = rn.Walkthrough(ctx, coach, tag)
	case "endcheck":
		r = rn.EndCheck(ctx, coach, tag)
	case "lookup":
		r = rn.Lookup(ctx, tag)
	case "legacy-cleanup":
		r = rn.LegacyCleanup(ctx)
	}
	emit(w, r)
	if r["result"] == "failed" {
		return 1
	}
	return 0
}
