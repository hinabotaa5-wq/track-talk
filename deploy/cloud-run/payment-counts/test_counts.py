import json
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


class CouponTest(unittest.TestCase):
    # 本体 #1031：クーポンを使った添削の決済（実額 = 価格 - クーポン）を価格不一致・孤児として数えない。
    def test_no_orphan_compares_payment_amount_to_price(self):
        for sql in counts.ORPHANS.values():
            self.assertNotIn("amount_yen =", sql)
            self.assertNotIn("base_price_yen", sql)

    def test_fully_subsidized_review_earning_is_not_an_orphan(self):
        sql = counts.ORPHANS["review_earning_without_payment_link"]
        self.assertIn("NOT (e.platform_subsidy_yen > 0 AND e.platform_subsidy_yen >= e.gross_yen)", sql)
        self.assertIn(("coach_earnings", "platform_subsidy_yen"), counts.REQUIRED_COLUMNS)

    def test_coupon_status_and_migration(self):
        self.assertEqual(counts.STATUS_COLUMNS[("coaching_coupons", "status")], ("unused", "reserved", "used", "expired", "canceled"))
        self.assertEqual(counts.EXPECTED_MIGRATION, 193)


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
