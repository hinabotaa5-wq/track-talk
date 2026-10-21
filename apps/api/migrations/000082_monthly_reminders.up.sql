-- 月1回の記録のお知らせ(プッシュ通知)を送った月。Cloud Scheduler が呼び直しても、同じ月に二度送らない。
CREATE TABLE monthly_reminders (
    month date PRIMARY KEY CHECK (extract(day FROM month) = 1),
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    recipients integer NOT NULL DEFAULT 0 CHECK (recipients >= 0)
);

ALTER TABLE monthly_reminders ENABLE ROW LEVEL SECURITY;
