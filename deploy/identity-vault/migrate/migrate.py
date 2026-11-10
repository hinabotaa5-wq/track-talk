"""Apply the identity vault migrations by command (ops #63).

Runs as the Cloud Run Job track-talk-identity-vault-migrate. The connection URL of the
vault's migration owner comes only from the job's secret reference (VAULT_MIGRATE_DATABASE_URL);
the person who starts the job never sees it. Every file in migrations/ is written to be
idempotent and is applied in name order, each in its own transaction. Afterwards a fixed set
of read-only checks runs. One JSON line is printed: file count, booleans and counts only.
Never prints SQL, database values, error text or connection details.
"""
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import urllib.parse

HERE = Path(__file__).resolve().parent
MIGRATIONS = HERE / "migrations" if (HERE / "migrations").is_dir() else HERE.parent / "migrations"
FILE = re.compile(r"\d{3}_[a-z0-9_]+\.sql")
LOCAL_HOSTS = ("localhost", "127.0.0.1", "::1", "host.docker.internal")
APP_NAME = "track-talk-identity-vault-migrate"

SYNTHETIC_FUNCTIONS = (
    "identity_vault.synthetic_bind(uuid,uuid)",
    "identity_vault.synthetic_status(uuid,uuid)",
    "identity_vault.synthetic_unbind(uuid)",
    "identity_vault.admin_readiness()",
    "identity_vault.legacy_unbind_aaa()",
)

CHECK_SQL = """
SELECT json_build_object(
  'phone_column', EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='identity_vault' AND table_name='identities' AND column_name='phone_enc'),
  'functions_present', (SELECT count(*) FROM unnest(ARRAY[{fns}, 'identity_vault.synthetic_subject(uuid)']) f WHERE to_regprocedure(f) IS NOT NULL),
  'functions_definer_pinned', (SELECT count(*) FROM pg_proc p WHERE p.oid IN (SELECT to_regprocedure(f) FROM unnest(ARRAY[{fns}, 'identity_vault.synthetic_subject(uuid)']) f)
                                AND p.prosecdef AND p.proconfig = ARRAY['search_path=pg_catalog']),
  'public_can_execute', (SELECT count(*) FROM pg_proc p CROSS JOIN LATERAL aclexplode(p.proacl) a
                          WHERE p.oid IN (SELECT to_regprocedure(f) FROM unnest(ARRAY[{fns}, 'identity_vault.synthetic_subject(uuid)']) f) AND a.grantee = 0),
  'restricted_roles_can_execute', (SELECT count(*) FROM pg_roles r, unnest(ARRAY[{fns}]) f
                          WHERE r.rolname IN ('anon','authenticated','service_role','identity_export') AND to_regprocedure(f) IS NOT NULL
                            AND has_function_privilege(r.oid, to_regprocedure(f), 'EXECUTE')),
  'app_roles', (SELECT count(DISTINCT a.grantee) FROM pg_class c CROSS JOIN LATERAL aclexplode(c.relacl) a JOIN pg_roles g ON g.oid = a.grantee
                 WHERE c.oid = 'identity_vault.identities'::regclass AND a.privilege_type='INSERT' AND a.grantee <> c.relowner AND g.rolcanlogin AND NOT g.rolsuper
                   AND g.rolname NOT IN ('anon','authenticated','service_role','identity_export')),
  'app_roles_can_execute_all', (SELECT count(*) FROM (SELECT DISTINCT a.grantee FROM pg_class c CROSS JOIN LATERAL aclexplode(c.relacl) a JOIN pg_roles g ON g.oid = a.grantee
                 WHERE c.oid = 'identity_vault.identities'::regclass AND a.privilege_type='INSERT' AND a.grantee <> c.relowner AND g.rolcanlogin AND NOT g.rolsuper
                   AND g.rolname NOT IN ('anon','authenticated','service_role','identity_export')) x
                 WHERE (SELECT bool_and(to_regprocedure(f) IS NOT NULL AND has_function_privilege(x.grantee, to_regprocedure(f), 'EXECUTE')) FROM unnest(ARRAY[{fns}]) f)),
  'members_admin_rows', (SELECT count(*) FROM identity_vault.members WHERE is_admin)
);
""".format(fns=", ".join("'" + f + "'" for f in SYNTHETIC_FUNCTIONS))


def pg_env(url, environ=None):
    environ = os.environ if environ is None else environ
    parsed = urllib.parse.urlsplit(url or "")
    if parsed.scheme not in ("postgres", "postgresql") or not parsed.hostname:
        raise RuntimeError("invalid_database_configuration")
    query = urllib.parse.parse_qs(parsed.query)
    sslmode = query.get("sslmode", ["require"])[0]
    if sslmode not in ("require", "verify-ca", "verify-full") and parsed.hostname not in LOCAL_HOSTS:
        raise RuntimeError("encrypted_database_connection_required")
    env = {k: v for k, v in environ.items() if k not in ("VAULT_MIGRATE_DATABASE_URL", "DATABASE_URL")}
    env.update(PGHOST=parsed.hostname, PGPORT=str(parsed.port or 5432),
               PGUSER=urllib.parse.unquote(parsed.username or ""), PGPASSWORD=urllib.parse.unquote(parsed.password or ""),
               PGDATABASE=urllib.parse.unquote(parsed.path.lstrip("/")) or "postgres", PGSSLMODE=sslmode,
               PGCONNECT_TIMEOUT="15", PGAPPNAME=APP_NAME, PGOPTIONS="-c statement_timeout=120000 -c lock_timeout=10000")
    if query.get("sslrootcert"):
        env["PGSSLROOTCERT"] = query["sslrootcert"][0]
    return env


def migration_files(directory=MIGRATIONS):
    files = sorted(p for p in directory.iterdir() if p.is_file() and p.suffix == ".sql")
    if not files or any(not FILE.fullmatch(p.name) for p in files):
        raise RuntimeError("unexpected_migration_file")
    return files


def psql(env, args, stdin=None):
    result = subprocess.run(["psql", "-X", "-q", "--no-password", "-v", "ON_ERROR_STOP=1", *args], input=stdin, env=env,
                            capture_output=True, text=True, timeout=300)
    if result.returncode != 0:
        # stderr can echo statements or connection details; it is never printed.
        raise RuntimeError("database_statement_failed")
    return result.stdout


def run(url, out=sys.stdout, environ=None, directory=MIGRATIONS):
    report = {"event": "identity_vault_migrate", "result": "failed"}
    code = 1
    try:
        env = pg_env(url, environ)
        files = migration_files(directory)
        report["files"] = len(files)
        applied = 0
        for path in files:
            psql(env, ["-1", "-f", str(path)])
            applied += 1
            report["applied"] = applied
        lines = [line for line in psql(env, ["-At"], stdin="BEGIN READ ONLY;\n" + CHECK_SQL + "ROLLBACK;\n").splitlines() if line.strip()]
        checks = json.loads(lines[-1])
        report["checks"] = {k: (v if isinstance(v, bool) else int(v)) for k, v in checks.items()}
        c = report["checks"]
        ok = (c["phone_column"] and c["functions_present"] == len(SYNTHETIC_FUNCTIONS) + 1
              and c["functions_definer_pinned"] == len(SYNTHETIC_FUNCTIONS) + 1 and c["public_can_execute"] == 0
              and c["restricted_roles_can_execute"] == 0 and c["app_roles"] >= 1 and c["app_roles_can_execute_all"] == c["app_roles"])
        if ok:
            report["result"], code = "ok", 0
        else:
            report["error"] = "post_check_mismatch"
    except RuntimeError as exc:
        report["error"] = str(exc) if re.fullmatch(r"[a-z_]+", str(exc)) else "failed"
    except Exception:  # noqa: BLE001 - never print details
        report["error"] = "failed"
    out.write(json.dumps(report, sort_keys=True) + "\n")
    return code


if __name__ == "__main__":
    sys.exit(run(os.environ.get("VAULT_MIGRATE_DATABASE_URL", "")))
