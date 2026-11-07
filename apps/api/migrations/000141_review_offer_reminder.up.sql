-- コーチング通知の欠落対応（本体 #420）。
-- 打診の引受期限前のリマインドを1回だけ送るため、送ったことを打診の行に残す。
ALTER TABLE review_offers
    ADD COLUMN reminded_at timestamptz;

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
        'review_offer_reminder',
        'review_accepted',
        'review_expired',
        'review_returned',
        'review_clarify',
        'review_clarified',
        'review_helpful_ask',
        'review_info_request',
        'review_resubmitted',
        'review_withdrawn',
        'message'
    ));
