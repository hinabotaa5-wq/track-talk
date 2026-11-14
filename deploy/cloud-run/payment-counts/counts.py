"""Read-only payment state counts. Prints one JSON line of counts; never prints rows, ids, amounts or credentials."""
import datetime as dt
import json
import os
import re
import subprocess
import sys
import urllib.parse

# Status enumerations as the code and migrations define them (checked through 000146).
STATUS_COLUMNS = {
    ("review_payments", "status"): ("pending", "authorized", "captured", "canceled", "refund_pending", "refunded"),
    ("coaching_payments", "status"): ("pending", "paid", "refunded", "partially_refunded"),
    ("coaching_ticket_purchases", "status"): ("pending", "paid", "partially_refunded", "refunded"),
    ("coaching_consultations", "payment_state"): ("unpaid", "paid", "refund_pending", "refunded"),
    ("coaching_contracts", "fulfillment_state"): ("open", "refund_pending", "refund_failed", "settled"),
    ("captured_payment_refunds", "status"): ("pending", "succeeded", "failed"),
    ("coaching_ticket_refunds", "status"): ("pending", "succeeded", "failed"),
    ("coaching_consultation_refunds", "status"): ("pending", "succeeded", "failed"),
    ("coach_payouts", "status"): ("pending", "paid", "failed"),
}

# Allowed by the schema, but must be zero while the refund features are closed.
FLAGGED_STATES = {
    ("review_payments", "status"): ("refund_pending",),
    ("coaching_consultations", "payment_state"): ("refund_pending",),
    ("coaching_contracts", "fulfillment_state"): ("refund_pending", "refund_failed"),
    ("captured_payment_refunds", "status"): ("pending", "failed"),
    ("coaching_ticket_refunds", "status"): ("pending", "failed"),
    ("coaching_consultation_refunds", "status"): ("pending", "failed"),
}

LINKED = "EXISTS (SELECT 1 FROM review_payment_earning_links l WHERE l.payment_id = p.id)"

# Orphan definitions; each is a single count(*). Sources are named in the ops report.
ORPHANS = {
    # captured_refund.go missingReviewRefundLink: the refund API refuses these.
    "review_payment_settled_unlinked":
        f"SELECT count(*) FROM review_payments p WHERE p.status IN ('captured','refund_pending','refunded') AND p.request_id IS NULL AND NOT {LINKED}",
    # A card hold whose request no longer exists.
    "review_payment_authorized_without_request":
        "SELECT count(*) FROM review_payments p WHERE p.status = 'authorized' AND p.request_id IS NULL",
    # queries/coach_payout.sql InsertReviewEarnings conditions, but no earning yet.
    "review_captured_returned_without_earning":
        "SELECT count(*) FROM review_payments p JOIN review_requests r ON r.id = p.request_id WHERE p.status = 'captured' AND r.returned_at IS NOT NULL AND r.coach_id IS NOT NULL AND NOT r.use_tickets AND NOT EXISTS (SELECT 1 FROM coach_earnings e WHERE e.review_request_id = r.id)",
    # Review earnings without a payment link and without a ticket reservation (candidates only).
    "review_earning_without_payment_link":
        "SELECT count(*) FROM coach_earnings e WHERE e.source = 'review' AND NOT EXISTS (SELECT 1 FROM review_payment_earning_links l WHERE l.earning_id = e.id) AND NOT EXISTS (SELECT 1 FROM coaching_ticket_reservations t WHERE t.request_id = e.review_request_id)",
    # coaching_payments.contract_id is ON DELETE SET NULL.
    "coaching_payment_settled_without_contract":
        "SELECT count(*) FROM coaching_payments p WHERE p.status <> 'pending' AND p.contract_id IS NULL",
    # coaching_consultations has no foreign key to its parent request or contract.
    "consultation_paid_without_parent":
        "SELECT count(*) FROM coaching_consultations c WHERE c.funding = 'cash' AND c.payment_state <> 'unpaid' AND NOT EXISTS (SELECT 1 FROM review_requests r WHERE r.id = c.review_request_id) AND NOT EXISTS (SELECT 1 FROM coaching_contracts k WHERE k.id = c.contract_id)",
    # queries/coaching_consultation.sql RecordConsultationEarnings conditions, but no earning.
    "consultation_answered_paid_without_earning":
        "SELECT count(*) FROM coaching_consultations c WHERE c.status = 'answered' AND c.funding = 'cash' AND c.payment_state = 'paid' AND c.coach_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM coach_earnings e WHERE e.consultation_id = c.id)",
    "ticket_purchase_paid_without_owner":
        "SELECT count(*) FROM coaching_ticket_purchases p WHERE p.status IN ('paid','partially_refunded') AND p.owner_id IS NULL",
    "captured_refund_without_payment":
        "SELECT count(*) FROM captured_payment_refunds f WHERE NOT EXISTS (SELECT 1 FROM review_payments p WHERE f.source = 'review' AND p.id = f.payment_id) AND NOT EXISTS (SELECT 1 FROM coaching_consultations c WHERE f.source = 'consultation' AND c.id = f.payment_id)",
}

# Informational only; not part of the gate.
INFO = {
    "review_payment_pending_session_open": "SELECT count(*) FROM review_payments p WHERE p.status = 'pending' AND p.session_expires_at > now()",
    "review_payment_unsettled_without_request": "SELECT count(*) FROM review_payments p WHERE p.status IN ('pending','canceled') AND p.request_id IS NULL",
    "coaching_payment_pending_session_open": "SELECT count(*) FROM coaching_payments p WHERE p.status = 'pending' AND p.session_expires_at > now()",
    "ticket_purchase_pending_session_open": "SELECT count(*) FROM coaching_ticket_purchases p WHERE p.status = 'pending' AND p.session_expires_at > now()",
}

ROW_COUNT_TABLES = ("review_payment_earning_links", "captured_refund_earnings", "coach_refund_adjustments",
                    "coach_refund_applications", "coach_refund_cancellations", "coach_refund_compensations",
                    "coach_refund_application_reversals")

REQUIRED_COLUMNS = sorted({*STATUS_COLUMNS, ("review_payments", "request_id"), ("review_payments", "session_expires_at"),
                           ("review_requests", "returned_at"), ("review_requests", "use_tickets"), ("review_requests", "coach_id"),
                           ("coach_earnings", "source"), ("coach_earnings", "review_request_id"), ("coach_earnings", "consultation_id"),
                           ("review_payment_earning_links", "payment_id"), ("review_payment_earning_links", "earning_id"),
                           ("coaching_payments", "contract_id"), ("coaching_payments", "session_expires_at"),
                           ("coaching_consultations", "funding"), ("coaching_consultations", "status"), ("coaching_consultations", "coach_id"),
                           ("coaching_consultations", "review_request_id"), ("coaching_consultations", "contract_id"),
                           ("coaching_ticket_purchases", "owner_id"), ("coaching_ticket_purchases", "session_expires_at"),
                           ("coaching_ticket_reservations", "request_id"), ("captured_payment_refunds", "source"),
                           ("captured_payment_refunds", "payment_id")} | {(t, "earning_id" if t in ("review_payment_earning_links", "captured_refund_earnings") else "id") for t in ROW_COUNT_TABLES})
EXPECTED_MIGRATION = 169

# row_security=off makes RLS raise an error instead of silently hiding rows from the count.
BEGIN = "BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;\nSET LOCAL row_security = off;\n"
NAME = re.compile(r"[a-z][a-z0-9_]{0,62}")
COUNT_SQL = re.compile(r"SELECT count\(\*\) FROM [^;]+")


def checked(sql):
    """Every count statement is a single SELECT count(*); anything else is a programming error."""
    if not COUNT_SQL.fullmatch(sql) or re.search(r"\b(insert|update|delete|alter|drop|create|grant|truncate|copy)\b", sql, re.I):
        raise RuntimeError("non_aggregate_statement")
    return sql


def pg_env(url):
    parsed = urllib.parse.urlsplit(url)
    if parsed.scheme not in ("postgres", "postgresql") or not parsed.hostname:
        raise RuntimeError("invalid_database_configuration")
    query = urllib.parse.parse_qs(parsed.query)
    sslmode = query.get("sslmode", ["require"])[0]
    local = parsed.hostname in ("127.0.0.1", "localhost", "host.docker.internal")
    if sslmode not in ("require", "verify-ca", "verify-full") and not local:
        raise RuntimeError("encrypted_database_connection_required")
    result = os.environ.copy()
    result.pop("DATABASE_URL", None)
    result.update(PGHOST=parsed.hostname, PGPORT=str(parsed.port or 5432),
                  PGUSER=urllib.parse.unquote(parsed.username or ""),
                  PGPASSWORD=urllib.parse.unquote(parsed.password or ""),
                  PGDATABASE=urllib.parse.unquote(parsed.path.lstrip("/")),
                  PGSSLMODE=sslmode, PGCONNECT_TIMEOUT="15", PGAPPNAME="track-talk-payment-counts",
                  PGOPTIONS="-c default_transaction_read_only=on -c statement_timeout=60000 -c lock_timeout=5000")
    if query.get("sslrootcert"):
        result["PGSSLROOTCERT"] = query["sslrootcert"][0]
    return result


def literal(value):
    if not re.fullmatch(r"[a-z][a-z0-9_.]{0,126}", value):
        raise RuntimeError("invalid_identifier")
    return "'" + value + "'"


def ident(value):
    if not NAME.fullmatch(value):
        raise RuntimeError("invalid_identifier")
    return value


def build_sql():
    pairs = ",".join(f"({literal(t)},{literal(c)})" for t, c in REQUIRED_COLUMNS)
    missing = ("SELECT coalesce(json_agg(r.t || '.' || r.c ORDER BY r.t, r.c), '[]') FROM (VALUES " + pairs + ") r(t,c) "
               "WHERE NOT EXISTS (SELECT 1 FROM information_schema.columns i WHERE i.table_schema = 'public' AND i.table_name = r.t AND i.column_name = r.c)")
    migration = "SELECT json_build_object('version', max(version), 'dirty', bool_or(dirty)) FROM schema_migrations"
    return (BEGIN + "SELECT json_build_object('read_only', current_setting('transaction_read_only'), 'migration', ("
            + migration + "), 'missing', (" + missing + "));\nROLLBACK;\n")


def counts_sql():
    parts = []
    for (table, column) in STATUS_COLUMNS:
        parts.append(f"{literal(table + '.' + column)}, (SELECT coalesce(json_object_agg(s, n), '{{}}') FROM "
                     f"(SELECT coalesce({ident(column)}::text, 'null') s, count(*) n FROM {ident(table)} GROUP BY 1) x)")
    for name, sql in {**ORPHANS, **INFO}.items():
        parts.append(f"{literal(name)}, ({checked(sql)})")
    for table in ROW_COUNT_TABLES:
        parts.append(f"{literal('rows.' + table)}, ({checked('SELECT count(*) FROM ' + table)})")
    return (BEGIN + "SELECT to_json(current_setting('transaction_read_only'));\n"
            "SELECT json_build_object(" + ", ".join(parts) + ");\nROLLBACK;\n")


def run_psql(env, sql):
    result = subprocess.run(["psql", "-XAtq", "--no-password", "-v", "ON_ERROR_STOP=1"], input=sql, env=env,
                            capture_output=True, text=True, timeout=120)
    if result.returncode != 0:
        # stderr can echo statement fragments or connection details; it is never logged.
        raise RuntimeError("database_query_failed")
    return [json.loads(line) for line in result.stdout.splitlines() if line.strip()]


def summarize(raw):
    """Turns raw query output into the report; only fixed names, enum names and integers survive."""
    status, unexpected, flagged = {}, {}, {}
    for (table, column), allowed in STATUS_COLUMNS.items():
        values = raw[table + "." + column]
        known = {v: int(values.get(v, 0)) for v in allowed}
        status.setdefault(table, {})[column] = known
        extra = {}
        for index, (value, n) in enumerate(sorted(values.items())):
            if value not in allowed:
                extra[value if NAME.fullmatch(value) else f"unprintable_value_{index}"] = int(n)
        if extra:
            unexpected[table + "." + column] = extra
        for value in FLAGGED_STATES.get((table, column), ()):
            if known[value]:
                flagged[table + "." + column + "=" + value] = known[value]
    orphans = {name: int(raw[name]) for name in ORPHANS}
    return {
        "status_counts": status,
        "unexpected_values": unexpected,
        "flagged_states": flagged,
        "orphans": orphans,
        "info": {name: int(raw[name]) for name in INFO},
        "row_counts": {t: int(raw["rows." + t]) for t in ROW_COUNT_TABLES},
        "result": "attention" if unexpected or flagged or any(orphans.values()) else "clear",
    }


def main():
    env = pg_env(os.environ["DATABASE_URL"])
    started = dt.datetime.now(dt.timezone.utc).isoformat()
    # The schema check runs first, so a mismatch is reported instead of failing inside the counts.
    rows = run_psql(env, build_sql())
    if len(rows) != 1 or rows[0]["read_only"] != "on":
        raise RuntimeError("read_only_transaction_required")
    head = rows[0]
    migration = {"version": head["migration"]["version"], "dirty": bool(head["migration"]["dirty"])}
    if head["missing"] or migration["dirty"] or migration["version"] != EXPECTED_MIGRATION:
        print(json.dumps({"severity": "WARNING", "event": "payment_counts_schema_mismatch", "generated_at": started,
                          "migration": migration, "missing_columns": [m for m in head["missing"] if re.fullmatch(r"[a-z0-9_.]+", m)]}), flush=True)
        sys.exit(2)
    rows = run_psql(env, counts_sql())
    if len(rows) != 2 or rows[0] != "on":
        raise RuntimeError("read_only_transaction_required")
    raw = rows[1]
    report = {"severity": "INFO", "event": "payment_counts", "generated_at": started, "read_only": True, "migration": migration, **summarize(raw)}
    print(json.dumps(report, ensure_ascii=False, sort_keys=True), flush=True)


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except Exception as error:
        reason = str(error) if isinstance(error, RuntimeError) and re.fullmatch(r"[a-z0-9_]{1,64}", str(error)) else None
        print(json.dumps({"severity": "ERROR", "event": "payment_counts_failed", "error_type": type(error).__name__, "reason": reason}), flush=True)
        sys.exit(1)
