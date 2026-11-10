"""Tests for migrate.py. Unit tests run anywhere; the integration test needs
TEST_VAULT_ADMIN_DATABASE_URL (a local vault-like database with the auth schema, schema.sql
applied and an application role holding INSERT on identity_vault.identities)."""
import io
import json
import os
from pathlib import Path
import tempfile
import unittest

import migrate


class UnitTests(unittest.TestCase):
    def test_requires_tls_for_remote(self):
        with self.assertRaisesRegex(RuntimeError, "encrypted_database_connection_required"):
            migrate.pg_env("postgres://u:p@db.example/postgres?sslmode=disable", {})
        env = migrate.pg_env("postgres://u:p%40x@db.example:6543/postgres?sslmode=require", {"VAULT_MIGRATE_DATABASE_URL": "x", "PATH": "/bin"})
        self.assertEqual((env["PGPASSWORD"], env["PGPORT"], env["PGAPPNAME"]), ("p@x", "6543", "track-talk-identity-vault-migrate"))
        self.assertNotIn("VAULT_MIGRATE_DATABASE_URL", env)
        migrate.pg_env("postgres://u:p@127.0.0.1/postgres?sslmode=disable", {})

    def test_rejects_bad_url_and_files(self):
        out = io.StringIO()
        self.assertEqual(migrate.run("mysql://u:secretpw@h/x", out=out, environ={}), 1)
        line = json.loads(out.getvalue())
        self.assertEqual(line["error"], "invalid_database_configuration")
        self.assertNotIn("secretpw", out.getvalue())
        with tempfile.TemporaryDirectory() as d:
            Path(d, "004_x.sql").write_text("SELECT 1;")
            Path(d, "notes.sql").write_text("SELECT 1;")
            with self.assertRaisesRegex(RuntimeError, "unexpected_migration_file"):
                migrate.migration_files(Path(d))

    def test_repo_migrations_are_listed_in_order(self):
        names = [p.name for p in migrate.migration_files()]
        self.assertEqual(names, sorted(names))
        self.assertIn("003_synthetic_provision.sql", names)

    def test_failed_statement_prints_no_detail(self):
        out = io.StringIO()
        code = migrate.run("postgres://nobody:secretpw@127.0.0.1:1/postgres?sslmode=disable", out=out, environ={"PATH": os.environ.get("PATH", "")})
        self.assertEqual(code, 1)
        self.assertNotIn("secretpw", out.getvalue())
        self.assertNotIn("127.0.0.1", out.getvalue())


@unittest.skipUnless(os.environ.get("TEST_VAULT_ADMIN_DATABASE_URL"), "no local vault database")
class IntegrationTests(unittest.TestCase):
    def test_apply_twice(self):
        url = os.environ["TEST_VAULT_ADMIN_DATABASE_URL"]
        for _ in range(2):
            out = io.StringIO()
            self.assertEqual(migrate.run(url, out=out), 0, out.getvalue())
            line = json.loads(out.getvalue())
            self.assertEqual(line["result"], "ok")
            self.assertEqual(line["applied"], line["files"])
            self.assertEqual(line["checks"]["public_can_execute"], 0)
            self.assertEqual(line["checks"]["restricted_roles_can_execute"], 0)
            self.assertGreaterEqual(line["checks"]["app_roles"], 1)


if __name__ == "__main__":
    unittest.main()
