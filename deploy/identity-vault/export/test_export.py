"""Tests for the identity vault export.

Unit tests always run. The database tests need a local PostgreSQL superuser
connection in TEST_ADMIN_DATABASE_URL (for example
postgres://tracktalk:tracktalk@localhost:5432/tracktalk?sslmode=disable); they
create throwaway databases and roles with synthetic ciphertext only and drop
them afterwards. Run: cd deploy/identity-vault/export && python3 -m unittest test_export
"""
import io
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import tempfile
import unittest
from unittest import mock
import urllib.parse

import export

HERE = Path(__file__).resolve().parent
SCHEMA_SQL = (HERE.parent / "schema.sql").read_text()
ROLE_SQL = (HERE.parent / "export-role.sql").read_text()
ADMIN_URL = os.environ.get("TEST_ADMIN_DATABASE_URL")
# Recognisable synthetic text. It is stored only as part of fake "ciphertext"
# blobs and must never appear in any log line.
MARKER = "合成テスト住所-SYNTHETIC"


def admin_env(database=None):
    env = export.local_env(ADMIN_URL)
    if database:
        env["PGDATABASE"] = database
    return env


def psql(env, sql):
    out = subprocess.run(["psql", "-XAtq", "-v", "ON_ERROR_STOP=1", "-c", sql], env=env,
                         stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, check=True)
    return out.stdout.strip()


def fake_ciphertext():
    # Shaped like a field-encrypted value (nonce + sealed bytes); never decryptable.
    return "\\x" + (secrets.token_bytes(12) + MARKER.encode() + secrets.token_bytes(16)).hex()


class UnitTest(unittest.TestCase):
    def test_refuses_when_key_is_configured(self):
        with self.assertRaises(RuntimeError) as caught:
            export.refuse_key_material({"COACH_IDENTITY_KEY": "x"})
        self.assertEqual(str(caught.exception), "key_material_present")
        export.refuse_key_material({"BACKUP_BUCKET": "b"})

    def test_vault_env_requires_tls_and_drops_key(self):
        with self.assertRaises(RuntimeError):
            export.vault_env("postgres://u:p@db.example.com/postgres?sslmode=disable")
        with mock.patch.dict(os.environ, {"COACH_IDENTITY_KEY": "x"}):
            env = export.vault_env("postgres://u:p@db.example.com/postgres")
        self.assertNotIn("COACH_IDENTITY_KEY", env)
        self.assertEqual(env["PGAPPNAME"], "track-talk-identity-export")
        self.assertIn("default_transaction_read_only=on", env["PGOPTIONS"])

    def test_verify_database_must_be_local(self):
        with self.assertRaises(RuntimeError) as caught:
            export.local_env("postgres://u:p@db.example.supabase.co:5432/postgres")
        self.assertEqual(str(caught.exception), "verify_database_must_be_local")

    def test_store_puts_manifest_last_under_fresh_prefix(self):
        with tempfile.TemporaryDirectory() as temp:
            for name in ("database.dump", "roles.sql", "manifest.json"):
                Path(temp, name).write_bytes(b"x")
            uploads = []
            first = export.store(temp, "bucket", lambda b, n, d, t: uploads.append(n), "token")
            second = export.store(temp, "bucket", lambda b, n, d, t: None, "token")
        self.assertTrue(first.startswith("identity-vault/"))
        self.assertNotEqual(first, second)
        self.assertEqual([n.rsplit("/", 1)[1] for n in uploads], ["database.dump", "roles.sql", "manifest.json"])

    def test_failed_upload_never_writes_manifest(self):
        uploads = []

        def flaky(bucket, name, data, token):
            if name.endswith("roles.sql"):
                raise RuntimeError("uploaded_object_checksum_mismatch")
            uploads.append(name)
        with tempfile.TemporaryDirectory() as temp:
            for name in ("database.dump", "roles.sql", "manifest.json"):
                Path(temp, name).write_bytes(b"x")
            with self.assertRaises(RuntimeError):
                export.store(temp, "bucket", flaky, "token")
        self.assertFalse(any(n.endswith("manifest.json") for n in uploads))

    def test_cli_failure_exits_nonzero_without_values(self):
        env = dict(os.environ, BACKUP_BUCKET="track-talk-test-bucket",
                   DATABASE_URL="postgres://user:hunter2-secret@db.example.com/postgres?sslmode=disable")
        env.pop("COACH_IDENTITY_KEY", None)
        out = subprocess.run([sys.executable, str(HERE / "export.py")], env=env, stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE, text=True)
        self.assertEqual(out.returncode, 1)
        line = json.loads(out.stdout)
        self.assertEqual(line, {"severity": "ERROR", "event": "identity_export_failed", "error_type": "RuntimeError",
                                "reason": "encrypted_database_connection_required"})
        self.assertNotIn("hunter2", out.stdout + out.stderr)

    def test_cli_refuses_key_before_connecting(self):
        env = dict(os.environ, COACH_IDENTITY_KEY="should-not-be-here", BACKUP_BUCKET="b-b-b",
                   DATABASE_URL="postgres://u:p@db.example.com/postgres")
        out = subprocess.run([sys.executable, str(HERE / "export.py")], env=env, stdout=subprocess.PIPE, text=True)
        self.assertEqual(out.returncode, 1)
        self.assertEqual(json.loads(out.stdout)["reason"], "key_material_present")
        self.assertNotIn("should-not-be-here", out.stdout)


@unittest.skipUnless(ADMIN_URL, "TEST_ADMIN_DATABASE_URL is not set")
class DatabaseTest(unittest.TestCase):
    def setUp(self):
        suffix = secrets.token_hex(4)
        self.vault = "vault_export_test_" + suffix
        self.restore = "vault_restore_test_" + suffix
        self.role = "identity_export_test_" + suffix
        # Throwaway login password for the CI server's password auth; never a real credential.
        self.password = secrets.token_hex(16)
        admin = admin_env()
        psql(admin, f"CREATE DATABASE {self.vault}")
        psql(admin, f"CREATE DATABASE {self.restore}")
        self.addCleanup(self.drop)
        owner = admin_env(self.vault)
        psql(owner, SCHEMA_SQL)
        psql(owner, ROLE_SQL.replace("identity_export", self.role))
        psql(owner, f"ALTER ROLE {self.role} PASSWORD '{self.password}'")
        self.seed(owner)

    def drop(self):
        admin = admin_env()
        for database in (self.vault, self.restore):
            psql(admin, f"DROP DATABASE IF EXISTS {database} WITH (FORCE)")
        for role in (self.role, self.role + "_filtered"):
            psql(admin, f"DO $$BEGIN IF EXISTS (SELECT FROM pg_roles WHERE rolname='{role}') THEN "
                        f"EXECUTE 'DROP OWNED BY {role} CASCADE'; EXECUTE 'DROP ROLE {role}'; END IF; END$$;")

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

    def export_env(self, role=None):
        env = admin_env(self.vault)
        env["PGUSER"] = role or self.role
        env["PGPASSWORD"] = self.password
        # Same session options as production (backup.pg_env), minus TLS for the local server.
        env["PGOPTIONS"] = "-c default_transaction_read_only=on -c statement_timeout=180000 -c lock_timeout=5000"
        return env

    def run_export(self, directory, role=None):
        return export.create_export(directory, self.export_env(role))

    def test_export_then_manifest_verification_passes(self):
        with tempfile.TemporaryDirectory() as temp:
            manifest = self.run_export(temp)
            self.assertEqual(manifest["kind"], "identity_vault")
            self.assertEqual(manifest["fields"], "as_stored_encrypted")
            counts = {t["table"]: t["rows"] for t in manifest["tables"]}
            self.assertEqual(counts, {"access_log": 3, "commerce": 3, "disclosures": 1, "identities": 3, "members": 3})
            self.assertTrue(all(m["rls"] for m in manifest["metadata"]))
            result = export.verify_export(temp, admin_env(self.restore))
        self.assertEqual(result, {"severity": "INFO", "event": "identity_export_verified", "tables": 5, "rows": 13})
        # Ciphertext survives byte for byte: the restored blobs equal the source blobs.
        source = psql(admin_env(self.vault), "SELECT md5(string_agg(address_enc::text, ',' ORDER BY coach_id)) FROM identity_vault.identities")
        restored = psql(admin_env(self.restore), "SELECT md5(string_agg(address_enc::text, ',' ORDER BY coach_id)) FROM identity_vault.identities")
        self.assertEqual(source, restored)

    def test_export_contains_only_the_vault_schema(self):
        psql(admin_env(self.vault), "CREATE TABLE public.unrelated (x int); INSERT INTO public.unrelated VALUES (1);")
        with tempfile.TemporaryDirectory() as temp:
            self.run_export(temp)
            listing = subprocess.run(["pg_restore", "--list", str(Path(temp, "database.dump"))],
                                     stdout=subprocess.PIPE, text=True, check=True).stdout
        self.assertIn("identity_vault identities", listing)
        self.assertNotIn("unrelated", listing)

    def test_tampered_dump_is_detected(self):
        with tempfile.TemporaryDirectory() as temp:
            self.run_export(temp)
            with Path(temp, "database.dump").open("ab") as dump:
                dump.write(b"\0")
            with self.assertRaises(RuntimeError) as caught:
                export.verify_export(temp, admin_env(self.restore))
        self.assertEqual(str(caught.exception), "database_checksum_mismatch")

    def test_manifest_difference_is_detected(self):
        with tempfile.TemporaryDirectory() as temp:
            self.run_export(temp)
            path = Path(temp, "manifest.json")
            manifest = json.loads(path.read_text())
            manifest["tables"][0]["rows"] += 1
            path.write_text(json.dumps(manifest))
            with self.assertRaises(RuntimeError) as caught:
                export.verify_export(temp, admin_env(self.restore))
        self.assertEqual(str(caught.exception), "manifest_mismatch")

    def test_role_hidden_by_row_security_fails_instead_of_exporting_nothing(self):
        filtered = self.role + "_filtered"
        owner = admin_env(self.vault)
        psql(owner, f"CREATE ROLE {filtered} LOGIN NOBYPASSRLS PASSWORD '{self.password}'; GRANT USAGE ON SCHEMA identity_vault TO {filtered}; "
                    f"GRANT SELECT ON ALL TABLES IN SCHEMA identity_vault TO {filtered};")
        with tempfile.TemporaryDirectory() as temp:
            with self.assertRaises(RuntimeError) as caught:
                self.run_export(temp, role=filtered)
            self.assertFalse(Path(temp, "manifest.json").exists())
        self.assertEqual(str(caught.exception), "database_snapshot_query_failed")

    def test_main_database_is_refused(self):
        with tempfile.TemporaryDirectory() as temp:
            env = admin_env(self.restore)  # empty database: no vault schema
            with self.assertRaises(RuntimeError) as caught:
                export.create_export(temp, env)
        self.assertEqual(str(caught.exception), "identity_vault_schema_missing")
        psql(admin_env(self.vault), "CREATE TABLE public.coach_profiles (id uuid)")
        with tempfile.TemporaryDirectory() as temp:
            with self.assertRaises(RuntimeError) as caught:
                export.create_export(temp, admin_env(self.vault))
        self.assertEqual(str(caught.exception), "main_database_refused")

    def test_export_role_cannot_write(self):
        with self.assertRaises(subprocess.CalledProcessError):
            psql(self.export_env(), "DELETE FROM identity_vault.access_log")
        env = self.export_env()
        env.pop("PGOPTIONS")  # even without the session option, the role default keeps it read-only
        with self.assertRaises(subprocess.CalledProcessError):
            psql(env, "DELETE FROM identity_vault.access_log")

    def test_run_export_logs_no_values(self):
        uploads = {}
        result = export.run_export(self.export_env(), "track-talk-test-bucket",
                                   lambda b, n, d, t: uploads.__setitem__(n.rsplit("/", 1)[1], d), lambda: "token")
        self.assertEqual(set(result), {"severity", "event", "prefix", "bytes", "tables", "rows"})
        self.assertEqual(result["event"], "identity_export_completed")
        self.assertEqual(result["rows"], 13)
        logged = json.dumps(result, ensure_ascii=False)
        self.assertNotIn(MARKER, logged)
        self.assertNotIn("合成試験", logged)
        manifest = json.loads(uploads["manifest.json"])
        self.assertNotIn(MARKER, json.dumps(manifest, ensure_ascii=False))
        self.assertEqual(manifest["database_bytes"], len(uploads["database.dump"]))

    def test_verify_cli_reports_mismatch_with_exit_code(self):
        with tempfile.TemporaryDirectory() as temp:
            self.run_export(temp)
            with Path(temp, "database.dump").open("ab") as dump:
                dump.write(b"\0")
            parsed = urllib.parse.urlsplit(ADMIN_URL)
            url = urllib.parse.urlunsplit(parsed._replace(path="/" + self.restore))
            env = dict(os.environ, VERIFY_DATABASE_URL=url)
            env.pop("COACH_IDENTITY_KEY", None)
            out = subprocess.run([sys.executable, str(HERE / "export.py"), "verify", temp], env=env,
                                 stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        self.assertEqual(out.returncode, 1)
        self.assertEqual(json.loads(out.stdout), {"severity": "ERROR", "event": "identity_export_verify_failed",
                                                  "error_type": "RuntimeError", "reason": "database_checksum_mismatch"})


if __name__ == "__main__":
    unittest.main()
