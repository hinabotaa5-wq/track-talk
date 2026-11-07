-- 通知の取りこぼし回復（本体 #421）。
-- 送るべき Push を永続化し、一時障害のあとに再送する。宛先単位ではなく依頼・配信単位で1行作り、
-- 送るたびにいまの購読に送り直す（購読が消えていたら後で登録された端末にも届く）。
CREATE TABLE push_outbox (
    id uuid PRIMARY KEY,
    profile_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    -- 送る内容。タップ先と tag は送り直しても同じにする（端末側で重ねない）。
    title text NOT NULL,
    body text NOT NULL,
    url text NOT NULL,
    tag text NOT NULL DEFAULT '',
    -- 受け取り設定の種類（important・review など）。送るたびにいまの設定を確かめる。
    category text NOT NULL,
    ttl_seconds integer NOT NULL CHECK (ttl_seconds > 0),
    -- 同じ出来事を二重に入れないための鍵（例：review_offer:<打診ID>）。送り直しはこの鍵で突き合わせる。
    dedup_key text NOT NULL UNIQUE,
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'sent', 'expired', 'discarded')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    -- ここを過ぎたら送らない（打診は引受期限、月1回は24時間、お知らせは72時間）。
    expires_at timestamptz NOT NULL,
    -- 直近の結果。送れなかった理由を名前で追えるようにする（sent・no_subscription・gone・failed・opted_out）。
    last_result text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz
);

CREATE INDEX push_outbox_pending_idx ON push_outbox (next_attempt_at)
WHERE status = 'pending';

ALTER TABLE push_outbox ENABLE ROW LEVEL SECURITY;

-- 月1回のお知らせは、ベルに入れ終えたかを残す。途中で止まって呼び直したとき、ベルを二重に入れない。
ALTER TABLE monthly_reminders
    ADD COLUMN in_app_done_at timestamptz;

-- 通知の許可が granted のまま端末の購読だけが消えたとき、自動で再購読してよい端末だったかを残す。
-- 購読の登録が1回でも成功したら真になり、購読の削除では戻さない。
ALTER TABLE profiles
    ADD COLUMN push_ever_subscribed boolean NOT NULL DEFAULT false;
