"""Tests for the identity vault keep-alive ping.

Unit tests always run. The database tests need a local PostgreSQL superuser
connection in TEST_ADMIN_DATABASE_URL (for example
postgres://tracktalk:tracktalk@localhost:5432/tracktalk?sslmode=disable); they
create throwaway databases and roles with synthetic ciphertext only and drop
them afterwards. Run: cd deploy/identity-vault/dbping && python3 -m unittest test_dbping
"""
import io
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import unittest
from unittest import mock

import dbping

HERE = Path(__file__).resolve().parent
SCHEMA_SQL = (HERE.parent / "schema.sql").read_text()
ROLE_SQL = (HERE.parent / "dbping-role.sql").read_text()
ADMIN_URL = os.environ.get("TEST_ADMIN_DATABASE_URL")
# Recognisable synthetic text. It is stored only as part of fake "ciphertext"
# blobs and must never appear in any log line.
MARKER = "合成テスト住所-SYNTHETIC"


LOCAL_HOSTS = ("localhost", "127.0.0.1", "::1")


def admin_env(database=None):
    """Connection settings for the throwaway test databases, which must be local (never production)."""
    import urllib.parse
    parsed = urllib.parse.urlsplit(ADMIN_URL)
    if parsed.scheme not in ("postgres", "postgresql") or parsed.hostname not in LOCAL_HOSTS:
        raise RuntimeError("test_database_must_be_local")
    env = dict(os.environ)
    env.update(PGHOST=parsed.hostname, PGPORT=str(parsed.port or 5432),
               PGUSER=urllib.parse.unquote(parsed.username or ""),
               PGPASSWORD=urllib.parse.unquote(parsed.password or ""),
               PGDATABASE=database or urllib.parse.unquote(parsed.path.lstrip("/")),
               PGSSLMODE="disable", PGCONNECT_TIMEOUT="15")
    return env


def psql(env, sql):
    out = subprocess.run(["psql", "-XAtq", "-v", "ON_ERROR_STOP=1", "-c", sql], env=env,
                         stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, check=True)
    return out.stdout.strip()


def fake_ciphertext():
    # Shaped like a field-encrypted value (nonce + sealed bytes); never decryptable.
    return "\\\\x" + (secrets.token_bytes(12) + MARKER.encode() + secrets.token_bytes(16)).hex()


class UnitTest(unittest.TestCase):
    def test_refuses_when_key_is_configured(self):
        with self.assertRaises(RuntimeError) as caught:
            dbping.refuse_key_material({"COACH_IDENTITY_KEY": "x"})
        self.assertEqual(str(caught.exception), "key_material_present")
        dbping.refuse_key_material({"DATABASE_URL": "u"})

    def test_vault_env_requires_tls_and_drops_key(self):
        with self.assertRaises(RuntimeError):
            dbping.vault_env("postgres://u:p@db.example.com/postgres?sslmode=disable")
        with mock.patch.dict(os.environ, {"COACH_IDENTITY_KEY": "x"}):
            env = dbping.vault_env("postgres://u:p@db.example.com/postgres")
        self.assertNotIn("COACH_IDENTITY_KEY", env)
        self.assertEqual(env["PGAPPNAME"], "track-talk-identity-dbping")
        self.assertIn("default_transaction_read_only=on", env["PGOPTIONS"])

    def test_cli_failure_exits_nonzero_without_values(self):
        env = dict(os.environ,
                   DATABASE_URL="postgres://user:hunter2-secret@db.example.com/postgres?sslmode=disable")
        env.pop("COACH_IDENTITY_KEY", None)
        out = subprocess.run([sys.executable, str(HERE / "dbping.py")], env=env, stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE, text=True)
        self.assertEqual(out.returncode, 1)
        line = json.loads(out.stdout)
        self.assertEqual(line, {"severity": "ERROR", "event": "identity_ping_failed", "error_type": "RuntimeError",
                                "reason": "encrypted_database_connection_required"})
        self.assertNotIn("hunter2", out.stdout + out.stderr)

    def test_cli_refuses_key_before_connecting(self):
        env = dict(os.environ, COACH_IDENTITY_KEY="should-not-be-here",
                   DATABASE_URL="postgres://u:p@db.example.com/postgres")
        out = subprocess.run([sys.executable, str(HERE / "dbping.py")], env=env, stdout=subprocess.PIPE, text=True)
        self.assertEqual(out.returncode, 1)
        self.assertEqual(json.loads(out.stdout)["reason"], "key_material_present")
        self.assertNotIn("should-not-be-here", out.stdout)


@unittest.skipUnless(ADMIN_URL, "TEST_ADMIN_DATABASE_URL is not set")
class DatabaseTest(unittest.TestCase):
    def setUp(self):
        suffix = secrets.token_hex(4)
        self.vault = "vault_dbping_test_" + suffix
        self.role = "identity_dbping_test_" + suffix
        # Throwaway login password for the CI server's password auth; never a real credential.
        self.password = secrets.token_hex(16)
        admin = admin_env()
        psql(admin, f"CREATE DATABASE {self.vault}")
        self.addCleanup(self.drop)
        owner = admin_env(self.vault)
        psql(owner, SCHEMA_SQL)
        psql(owner, ROLE_SQL.replace("identity_dbping", self.role))
        psql(owner, f"ALTER ROLE {self.role} PASSWORD '{self.password}'")
        self.seed(owner)

    def drop(self):
        admin = admin_env()
        psql(admin, f"DROP DATABASE IF EXISTS {self.vault} WITH (FORCE)")
        psql(admin, f"DO $$BEGIN IF EXISTS (SELECT FROM pg_roles WHERE rolname='{self.role}') THEN "
                    f"EXECUTE 'DROP OWNED BY {self.role} CASCADE'; EXECUTE 'DROP ROLE {self.role}'; END IF; END$$;")

    def seed(self, env):
        rows = []
        for i in range(3):
            coach = f"00000000-0000-4000-8000-00000000000{i}"
            rows.append(f"INSERT INTO identity_vault.members VALUES (gen_random_uuid(), '{coach}', false);")
            rows.append(f"INSERT INTO identity_vault.identities (coach_id, key_id, legal_name_enc, address_enc) "
                        f"VALUES ('{coach}', 'test-key-1', '{fake_ciphertext()}', '{fake_ciphertext()}');")
            rows.append(f"INSERT INTO identity_vault.commerce (coach_id) VALUES ('{coach}');")
            rows.append(f"INSERT INTO identity_vault.access_log (coach_id, viewer, purpose) "
                        f"VALUES ('{coach}', gen_random_uuid(), '合成試験');")
        rows.append("INSERT INTO identity_vault.disclosures VALUES (gen_random_uuid(), "
                    "'00000000-0000-4000-8000-000000000000', gen_random_uuid(), gen_random_uuid());")
        psql(env, "".join(rows))

    def ping_env(self):
        env = admin_env(self.vault)
        env["PGUSER"] = self.role
        env["PGPASSWORD"] = self.password
        # Same session options as production (backup.pg_env), minus TLS for the local server.
        env["PGOPTIONS"] = "-c default_transaction_read_only=on -c statement_timeout=180000 -c lock_timeout=5000"
        return env

    def test_ping_returns_counts_only(self):
        outcome = dbping.ping(self.ping_env())
        self.assertEqual({t["table"] for t in outcome["tables"]},
                         {"members", "identities", "commerce", "disclosures", "access_log"})
        self.assertEqual(sum(t["rows"] for t in outcome["tables"]), 13)
        logged = json.dumps(outcome, ensure_ascii=False)
        self.assertNotIn(MARKER, logged)
        self.assertNotIn("合成試験", logged)

    def test_main_database_is_refused(self):
        with self.assertRaises(RuntimeError) as caught:
            dbping.ping(admin_env("postgres"))  # system database: no vault schema
        self.assertEqual(str(caught.exception), "identity_vault_schema_missing")
        psql(admin_env(self.vault), "CREATE TABLE public.coach_profiles (id uuid)")
        with self.assertRaises(RuntimeError) as caught:
            dbping.ping(self.ping_env())
        self.assertEqual(str(caught.exception), "main_database_refused")

    def test_role_cannot_write(self):
        with self.assertRaises(subprocess.CalledProcessError):
            psql(self.ping_env(), "DELETE FROM identity_vault.access_log")
        env = self.ping_env()
        env.pop("PGOPTIONS")  # even without the session option, the role default keeps it read-only
        with self.assertRaises(subprocess.CalledProcessError):
            psql(env, "DELETE FROM identity_vault.access_log")


if __name__ == "__main__":
    unittest.main()
