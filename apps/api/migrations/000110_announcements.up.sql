-- 運営から全員へのお知らせ(管理者画面から送る)。1回の送信が1行で、id は管理者画面が送信の確認を開いたときに決める。
-- 同じ id でもう一度送られたら(ボタンの二度押し・通信の再送)、二度目は送らない。
CREATE TABLE announcements (
    id uuid PRIMARY KEY,
    sender_id uuid REFERENCES profiles (id) ON DELETE SET NULL,
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 40),
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 200),
    url text NOT NULL CHECK (url LIKE '/%' AND url NOT LIKE '//%' AND char_length(url) <= 200),
    created_at timestamptz NOT NULL DEFAULT now(),
    in_app_count integer NOT NULL DEFAULT 0 CHECK (in_app_count >= 0),
    finished_at timestamptz,
    push_recipients integer NOT NULL DEFAULT 0 CHECK (push_recipients >= 0),
    -- 取り消したら、ベルからは消える。プッシュは端末に届いたあとなので取り消せない。
    canceled_at timestamptz
);

ALTER TABLE announcements ENABLE ROW LEVEL SECURITY;

-- ベルのお知らせがどの送信のものか。取り消すときに、まとめて消す。
ALTER TABLE notifications
    ADD COLUMN announcement_id uuid REFERENCES announcements (id) ON DELETE CASCADE;
CREATE INDEX notifications_announcement_idx ON notifications (announcement_id) WHERE announcement_id IS NOT NULL;

ALTER TABLE notification_settings DROP CONSTRAINT notification_settings_category_check;
ALTER TABLE notification_settings
    ADD CONSTRAINT notification_settings_category_check CHECK (category IN (
        'comment',
        'practice',
        'recruitment',
        'injury',
        'review',
        'coaching',
        'monthly',
        'announcement'
    ));
