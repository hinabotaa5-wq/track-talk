import json
import sqlite3
import unittest

import counts


def raw_counts(**overrides):
    raw = {t + "." + c: {} for t, c in counts.STATUS_COLUMNS}
    raw.update({name: 0 for name in {**counts.ORPHANS, **counts.INFO}})
    raw.update({"rows." + t: 0 for t in counts.ROW_COUNT_TABLES})
    raw.update(overrides)
    return raw


def leaves(value):
    if isinstance(value, dict):
        for k, v in value.items():
            yield k
            yield from leaves(v)
    else:
        yield value


class StatementTest(unittest.TestCase):
    def test_every_statement_is_a_count(self):
        for sql in {**counts.ORPHANS, **counts.INFO}.values():
            self.assertEqual(counts.checked(sql), sql)

    def test_review_earning_can_match_retained_reservation_after_withdrawal(self):
        sql = counts.ORPHANS["review_earning_without_payment_link"]
        with sqlite3.connect(":memory:") as db:
            db.executescript("""
                CREATE TABLE coach_earnings (id INTEGER, source TEXT, review_request_id INTEGER, reservation_id INTEGER);
                CREATE TABLE review_payment_earning_links (earning_id INTEGER);
                CREATE TABLE coaching_ticket_reservations (id INTEGER, request_id INTEGER);
                INSERT INTO coach_earnings VALUES
                    (1, 'review', 11, 10), (2, 'review', 21, 20), (3, 'review', 31, NULL),
                    (4, 'review', 41, NULL), (5, 'review', 51, NULL), (6, 'consultation', NULL, NULL);
                INSERT INTO review_payment_earning_links VALUES (3);
                INSERT INTO coaching_ticket_reservations VALUES (10, 11), (20, 21), (40, 41);
            """)
            self.assertEqual(db.execute(sql).fetchone(), (1,))
            db.execute("UPDATE coaching_ticket_reservations SET request_id = NULL WHERE id IN (10, 20)")
            self.assertEqual(db.execute(sql).fetchone(), (1,))

    def test_rejects_non_aggregate_statements(self):
        for sql in ("SELECT id FROM review_payments", "SELECT count(*) FROM t; DELETE FROM t",
                    "SELECT count(*) FROM t WHERE x IN (UPDATE t SET a=1)"):
            with self.assertRaises(RuntimeError):
                counts.checked(sql)

    def test_counts_sql_is_read_only_and_rolled_back(self):
        sql = counts.counts_sql()
        self.assertTrue(sql.startswith("BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;"))
        self.assertTrue(sql.endswith("ROLLBACK;\n"))
        self.assertIn("SET LOCAL row_security = off;", sql)


class SummarizeTest(unittest.TestCase):
    def test_clear_when_all_zero(self):
        self.assertEqual(counts.summarize(raw_counts())["result"], "clear")

    def test_flagged_and_unexpected_states(self):
        report = counts.summarize(raw_counts(**{"review_payments.status": {"captured": 2, "refund_pending": 1, "disputed": 3, "Bad Value!": 1}}))
        self.assertEqual(report["status_counts"]["review_payments"]["status"]["captured"], 2)
        self.assertEqual(report["flagged_states"], {"review_payments.status=refund_pending": 1})
        self.assertEqual(report["unexpected_values"], {"review_payments.status": {"disputed": 3, "unprintable_value_0": 1}})
        self.assertEqual(report["result"], "attention")

    def test_orphan_sets_attention(self):
        self.assertEqual(counts.summarize(raw_counts(review_payment_settled_unlinked=1))["result"], "attention")

    def test_output_holds_only_names_and_integers(self):
        report = counts.summarize(raw_counts(**{"coach_payouts.status": {"paid": 4}}))
        for leaf in leaves(report):
            self.assertTrue(isinstance(leaf, int) or (isinstance(leaf, str) and counts.re.fullmatch(r"[a-z0-9_.=]+", leaf)), leaf)
        json.dumps(report)


class ConnectionTest(unittest.TestCase):
    def test_requires_encryption_for_remote_hosts(self):
        with self.assertRaises(RuntimeError):
            counts.pg_env("postgresql://u:p@db.example.com:5432/postgres?sslmode=disable")

    def test_sets_read_only_session_and_drops_url(self):
        env = counts.pg_env("postgresql://u:p@db.example.com:5432/postgres")
        self.assertIn("default_transaction_read_only=on", env["PGOPTIONS"])
        self.assertNotIn("DATABASE_URL", env)


if __name__ == "__main__":
    unittest.main()
