"""Read-only export of the identity vault database to the backup bucket.

The rows are exported exactly as stored: identity fields stay encrypted, nothing
is decrypted and no key is ever loaded. It reuses the main database backup
(deploy/cloud-run/db-backup/backup.py) for the connection settings, the
read-only snapshot, the upload with checksum check and the failure logging, and
stores the set the same way: dump, roles, then manifest.json as the completion
marker, under a fresh prefix that is never overwritten.

Never prints database values or credentials.
"""
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import urllib.parse
import uuid

_HERE = Path(__file__).resolve().parent
_SHARED = _HERE.parent.parent / "cloud-run" / "db-backup"
if not (_HERE / "backup.py").exists() and _SHARED.is_dir():
    sys.path.insert(0, str(_SHARED))  # source tree; in the image both files sit in /app

import backup  # noqa: E402

SCHEMA = "identity_vault"
PREFIX = "identity-vault/"
APP_NAME = "track-talk-identity-export"
# The exporter must never be able to decrypt. Refuse to run where the key is configured.
KEY_VARIABLES = ("COACH_IDENTITY_KEY",)
LOCAL_HOSTS = ("localhost", "127.0.0.1", "::1")


def refuse_key_material(environ=None):
    environ = os.environ if environ is None else environ
    if any(name in environ for name in KEY_VARIABLES):
        raise RuntimeError("key_material_present")


def vault_env(url):
    env = backup.pg_env(url)  # requires TLS, read-only transactions, timeouts
    env["PGAPPNAME"] = APP_NAME
    for name in KEY_VARIABLES:
        env.pop(name, None)
    return env


def audit(query):
    """Same formula as the main backup audit (rows + digest of row digests), restricted to the vault schema."""
    tables = [json.loads(x)[0] for x in query(
        "SELECT json_build_array(c.relname) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace "
        f"WHERE c.relkind='r' AND n.nspname='{SCHEMA}' ORDER BY c.relname;")]
    result = []
    for table in tables:
        identifier = '"' + SCHEMA + '"."' + table.replace('"', '""') + '"'
        row = json.loads(query(
            "SELECT json_build_object('rows',count(*),'digest',md5(coalesce(string_agg(md5(to_jsonb(t)::text),'' "
            f"ORDER BY md5(to_jsonb(t)::text)),''))) FROM {identifier} t;")[0])
        result.append(dict(schema=SCHEMA, table=table, **row))
    return result


def metadata(query):
    return json.loads(query(
        "SELECT coalesce(jsonb_agg(x ORDER BY x.name),'[]'::jsonb) FROM (SELECT c.relname AS name,"
        "c.relrowsecurity AS rls,c.relforcerowsecurity AS force_rls,"
        "(SELECT count(*) FROM pg_policy p WHERE p.polrelid=c.oid) AS policies,"
        "(SELECT count(*) FROM pg_constraint k WHERE k.conrelid=c.oid) AS constraints,"
        "(SELECT count(*) FROM pg_index i WHERE i.indrelid=c.oid) AS indexes "
        f"FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind='r' AND n.nspname='{SCHEMA}') x;")[0])


def create_export(directory, env):
    """Dump the vault schema from one read-only snapshot and write manifest.json. Returns the manifest."""
    directory = Path(directory)
    with (directory / "private-errors.log").open("w+") as errors:
        snapshot = backup.Snapshot(env, errors)
        try:
            # Only the vault database: it must carry the vault schema and must not be the main database.
            checks = json.loads(snapshot.query(
                f"SELECT json_build_array(to_regnamespace('{SCHEMA}') IS NOT NULL, to_regclass('public.coach_profiles') IS NULL);")[0])
            if not checks[0]:
                raise RuntimeError("identity_vault_schema_missing")
            if not checks[1]:
                raise RuntimeError("main_database_refused")
            # Row security stays off (pg_dump's default), so a role that RLS would filter fails
            # loudly instead of producing an empty export.
            snapshot.query("SET row_security = off;")
            tables = audit(snapshot.query)
            if not tables:
                raise RuntimeError("identity_vault_tables_missing")
            dump = directory / "database.dump"
            with dump.open("wb") as output:
                process = subprocess.Popen(
                    ["pg_dump", "--format=custom", "--compress=6", "--lock-wait-timeout=5s",
                     "--schema=" + SCHEMA, "--snapshot=" + snapshot.info["snapshot"]],
                    env=env, stdout=subprocess.PIPE, stderr=errors)
                try:
                    while chunk := process.stdout.read(1024 * 1024):
                        output.write(chunk)
                        if output.tell() > backup.MAX_BYTES:
                            raise RuntimeError("backup_size_limit_exceeded")
                    if process.wait(timeout=10) != 0:
                        raise RuntimeError("database_dump_failed")
                finally:
                    if process.poll() is None:
                        process.kill()
                        process.wait()
                    process.stdout.close()
            if subprocess.run(["pg_restore", "--file=/dev/null", str(dump)], stderr=errors, timeout=60).returncode:
                raise RuntimeError("archive_validation_failed")
            roles = directory / "roles.sql"
            with roles.open("wb") as output:
                if subprocess.run(["pg_dumpall", "--roles-only", "--no-role-passwords"], env=env,
                                  stdout=output, stderr=errors, timeout=30).returncode:
                    raise RuntimeError("role_backup_failed")
            manifest = dict(format_version=1, kind="identity_vault",
                            created_at=dt.datetime.now(dt.timezone.utc).isoformat(),
                            server_version=snapshot.info["server_version"], read_only=True,
                            fields="as_stored_encrypted", schema=SCHEMA,
                            database_bytes=dump.stat().st_size,
                            database_sha256=hashlib.sha256(dump.read_bytes()).hexdigest(),
                            roles_sha256=hashlib.sha256(roles.read_bytes()).hexdigest(),
                            tables=tables, metadata=metadata(snapshot.query))
            (directory / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
            return manifest
        finally:
            snapshot.close()


def store(directory, bucket, upload_fn, access_token, now=None):
    """Upload dump, roles, then the manifest. Without the manifest the set is incomplete and never used."""
    now = now or dt.datetime.now(dt.timezone.utc)
    prefix = PREFIX + now.strftime("%Y-%m-%d/%Y%m%dT%H%M%SZ") + "-" + uuid.uuid4().hex
    for filename in ("database.dump", "roles.sql", "manifest.json"):
        upload_fn(bucket, prefix + "/" + filename, (Path(directory) / filename).read_bytes(), access_token)
    return prefix


def run_export(env, bucket, upload_fn, token_fn):
    with tempfile.TemporaryDirectory(prefix="tracktalk-identity-export-") as temp:
        manifest = create_export(temp, env)
        prefix = store(temp, bucket, upload_fn, token_fn())
    return {"severity": "INFO", "event": "identity_export_completed", "prefix": prefix,
            "bytes": manifest["database_bytes"], "tables": len(manifest["tables"]),
            "rows": sum(t["rows"] for t in manifest["tables"])}


def local_env(url):
    """Connection settings for the verification database, which must be local (never production)."""
    parsed = urllib.parse.urlsplit(url)
    if parsed.scheme not in ("postgres", "postgresql") or parsed.hostname not in LOCAL_HOSTS:
        raise RuntimeError("verify_database_must_be_local")
    env = os.environ.copy()
    env.pop("VERIFY_DATABASE_URL", None)
    env.update(PGHOST=parsed.hostname, PGPORT=str(parsed.port or 5432),
               PGUSER=urllib.parse.unquote(parsed.username or ""),
               PGPASSWORD=urllib.parse.unquote(parsed.password or ""),
               PGDATABASE=urllib.parse.unquote(parsed.path.lstrip("/")),
               PGSSLMODE="disable", PGCONNECT_TIMEOUT="15", PGAPPNAME=APP_NAME + "-verify")
    return env


def verify_export(directory, env):
    """Restore the set into an empty local database and compare it with its manifest.

    Checks the dump checksum, then that every table's row count and row digest match.
    Raises RuntimeError with a fixed reason on any difference. Prints no values.
    """
    directory = Path(directory)
    manifest = json.loads((directory / "manifest.json").read_text())
    dump = directory / "database.dump"
    if hashlib.sha256(dump.read_bytes()).hexdigest() != manifest["database_sha256"]:
        raise RuntimeError("database_checksum_mismatch")
    roles = directory / "roles.sql"
    if roles.exists() and hashlib.sha256(roles.read_bytes()).hexdigest() != manifest["roles_sha256"]:
        raise RuntimeError("roles_checksum_mismatch")
    with tempfile.TemporaryFile("w+") as errors:
        if subprocess.run(["pg_restore", "--no-owner", "--no-acl", "--exit-on-error", "--dbname=" + env["PGDATABASE"], str(dump)],
                          env=env, stderr=errors, stdout=subprocess.DEVNULL, timeout=300).returncode:
            raise RuntimeError("restore_failed")

        def query(sql):
            out = subprocess.run(["psql", "-XAtq", "--no-password", "-v", "ON_ERROR_STOP=1", "-c", sql],
                                 env=env, stdout=subprocess.PIPE, stderr=errors, text=True, timeout=120)
            if out.returncode:
                raise RuntimeError("verify_query_failed")
            return [line for line in out.stdout.splitlines() if line]

        restored = {t["table"]: t for t in audit(query)}
    expected = {t["table"]: t for t in manifest["tables"]}
    if set(restored) != set(expected):
        raise RuntimeError("manifest_table_set_mismatch")
    for name, table in expected.items():
        if restored[name]["rows"] != table["rows"] or restored[name]["digest"] != table["digest"]:
            raise RuntimeError("manifest_mismatch")
    return {"severity": "INFO", "event": "identity_export_verified", "tables": len(expected),
            "rows": sum(t["rows"] for t in expected.values())}


def main(argv):
    os.umask(0o077)
    refuse_key_material()
    if len(argv) == 3 and argv[1] == "verify":
        # Local check only: python3 export.py verify <directory>, with VERIFY_DATABASE_URL an empty local database.
        return verify_export(argv[2], local_env(os.environ["VERIFY_DATABASE_URL"]))
    if len(argv) != 1:
        raise RuntimeError("usage")
    bucket = os.environ["BACKUP_BUCKET"]
    if not re.fullmatch(r"[a-z0-9][a-z0-9._-]{1,220}[a-z0-9]", bucket):
        raise RuntimeError("invalid_bucket_configuration")
    env = vault_env(os.environ["DATABASE_URL"])
    return run_export(env, bucket, backup.upload, backup.token)


if __name__ == "__main__":
    verifying = len(sys.argv) > 1 and sys.argv[1] == "verify"
    try:
        print(json.dumps(main(sys.argv), ensure_ascii=False), flush=True)
    except Exception as error:
        # Do not log exception text: database clients and HTTP errors can contain secrets or data.
        event = "identity_export_verify_failed" if verifying else "identity_export_failed"
        print(json.dumps({"severity": "ERROR", "event": event, **backup.failure_details(error)}), flush=True)
        sys.exit(1)
