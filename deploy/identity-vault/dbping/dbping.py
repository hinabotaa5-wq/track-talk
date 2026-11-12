"""Vault DB keep-alive ping (H3): light read-only read, counts only.

Runs SELECT count(*) per table in the identity_vault schema and exits 0 with
an identity_ping_completed log line (table/row numbers only). It never prints
database values or credentials, never writes, never exports, and never loads
the decryption key.

The free-tier pause guard (PREMISES section 4) needs reads several times a day;
the existing /health keep-alive does not touch the database, and the daily
export runs only once. This job is the database-touching side, triggered by
Cloud Scheduler several times a day (see ../dbping-schedule.sh, paused on
creation, resumed only after a successful manual run).

Never prints database values or credentials.
"""
import datetime as dt
import json
import os
from pathlib import Path
import sys
import urllib.parse

_HERE = Path(__file__).resolve().parent
_SHARED = _HERE.parent.parent / "cloud-run" / "db-backup"
if not (_HERE / "backup.py").exists() and _SHARED.is_dir():
    sys.path.insert(0, str(_SHARED))  # source tree; in the image both files sit in /app

import backup  # noqa: E402

SCHEMA = "identity_vault"
APP_NAME = "track-talk-identity-dbping"
# The ping must never be able to decrypt. Refuse to run where the key is configured.
KEY_VARIABLES = ("COACH_IDENTITY_KEY",)


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


def ping(env):
    """Count rows per table in the vault schema from one read-only snapshot.

    Returns {"tables": [{"table": name, "rows": n}], "server_version": ...}.
    Counts only; no row values or digests ever leave the database.
    """
    with open(os.devnull, "w") as errors:
        snapshot = backup.Snapshot(env, errors)
        try:
            # Only the vault database: it must carry the vault schema and must not be the main database.
            checks = json.loads(snapshot.query(
                f"SELECT json_build_array(to_regnamespace('{SCHEMA}') IS NOT NULL, to_regclass('public.coach_profiles') IS NULL);")[0])
            if not checks[0]:
                raise RuntimeError("identity_vault_schema_missing")
            if not checks[1]:
                raise RuntimeError("main_database_refused")
            tables = [json.loads(x)[0] for x in snapshot.query(
                "SELECT json_build_array(c.relname) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace "
                f"WHERE c.relkind='r' AND n.nspname='{SCHEMA}' ORDER BY c.relname;")]
            if not tables:
                raise RuntimeError("identity_vault_tables_missing")
            result = []
            for table in tables:
                identifier = '"' + SCHEMA + '"."' + table.replace('"', '""') + '"'
                rows = json.loads(snapshot.query(
                    f"SELECT count(*) FROM {identifier} t;")[0])
                result.append({"table": table, "rows": rows})
            return {"tables": result, "server_version": snapshot.info["server_version"]}
        finally:
            snapshot.close()


def main(argv):
    os.umask(0o077)
    refuse_key_material()
    if len(argv) != 1:
        raise RuntimeError("usage")
    env = vault_env(os.environ["DATABASE_URL"])
    outcome = ping(env)
    return {"severity": "INFO", "event": "identity_ping_completed",
            "tables": len(outcome["tables"]), "rows": sum(t["rows"] for t in outcome["tables"]),
            "checked_at": dt.datetime.now(dt.timezone.utc).isoformat()}


if __name__ == "__main__":
    try:
        print(json.dumps(main(sys.argv), ensure_ascii=False), flush=True)
    except Exception as error:
        # Do not log exception text: database clients and HTTP errors can contain secrets or data.
        print(json.dumps({"severity": "ERROR", "event": "identity_ping_failed",
                          **backup.failure_details(error)}), flush=True)
        sys.exit(1)
