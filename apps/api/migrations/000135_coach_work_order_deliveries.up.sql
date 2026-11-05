-- 取引条件(作業指示書)をコーチへ宛先付きで届けた記録(フリーランス保護法3条の電磁的方法による明示)。
-- 引き受けで coach_work_orders を書き留めるのと同じトランザクションで、コーチのお知らせ(ベル)に
-- 条件の要点と全文を見られる画面の URL を入れ、ここに宛先・日時・送った文面を残す。
-- お知らせ自体は古くなると消えるので、送った文面はこの表に写して持つ。後から書き換えない。
CREATE TABLE coach_work_order_deliveries (
    id uuid PRIMARY KEY,
    work_order_id uuid NOT NULL UNIQUE REFERENCES coach_work_orders (id) ON DELETE CASCADE,
    recipient_id uuid REFERENCES profiles (id) ON DELETE SET NULL,
    channel text NOT NULL CHECK (channel IN ('in_app')),
    terms_version smallint NOT NULL,
    title text NOT NULL,
    body text NOT NULL,
    url text NOT NULL,
    delivered_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX coach_work_order_deliveries_recipient_idx ON coach_work_order_deliveries (recipient_id, delivered_at DESC);
ALTER TABLE coach_work_order_deliveries ENABLE ROW LEVEL SECURITY;
