-- 通知の種類ごとの受け取り設定。行がない種類は、アプリの決めた初期値(API の notificationCategories)に従う。
-- in_app はベルの一覧、push は通知をオンにした端末へのプッシュ。
CREATE TABLE notification_settings (
    profile_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    category text NOT NULL CHECK (category IN (
        'comment',
        'practice',
        'recruitment',
        'injury',
        'review',
        'coaching',
        'monthly'
    )),
    in_app boolean NOT NULL,
    push boolean NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (profile_id, category)
);

ALTER TABLE notification_settings ENABLE ROW LEVEL SECURITY;

-- これまでプッシュだけだった知らせ(パーソナル指導、支払い、月1回のお知らせなど)もベルに出す。
-- 投稿や添削の依頼にひもづかないので、見出し・本文・開く場所をそのまま持つ。
ALTER TABLE notifications
    ADD COLUMN title text,
    ADD COLUMN body text,
    ADD COLUMN url text;

ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_kind_check CHECK (kind IN (
        'recruitment_post',
        'comment',
        'comment_reply',
        'practice_application',
        'practice_accepted',
        'practice_rate',
        'injury_update',
        'review_offer',
        'review_expired',
        'review_returned',
        'review_clarify',
        'review_clarified',
        'review_helpful_ask',
        'message'
    ));
ALTER TABLE notifications DROP CONSTRAINT notifications_target_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_target_check CHECK (post_id IS NOT NULL OR review_request_id IS NOT NULL OR kind = 'message');
ALTER TABLE notifications
    ADD CONSTRAINT notifications_message_check CHECK (
        kind <> 'message' OR (title IS NOT NULL AND body IS NOT NULL AND url LIKE '/%')
    );
