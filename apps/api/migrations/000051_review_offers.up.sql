-- 添削の依頼をコーチに回す（割り当てと打診）。流れは online-coaching/design-review.md の「4. 割り当ての仕組み」のとおり。

-- 打診の履歴。同じ依頼を同じコーチに二度は回さない。result が NULL のものが返事待ち。
CREATE TABLE review_offers (
    id uuid PRIMARY KEY,
    request_id uuid NOT NULL REFERENCES review_requests (id) ON DELETE CASCADE,
    coach_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    offered_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    result text,
    answered_at timestamptz,
    UNIQUE (request_id, coach_id),
    CHECK (result IN ('accepted', 'declined', 'expired')),
    CHECK ((result IS NULL) = (answered_at IS NULL))
);

-- 1つの依頼に返事待ちの打診は1つだけ。
CREATE UNIQUE INDEX review_offers_open_idx ON review_offers (request_id) WHERE result IS NULL;
CREATE INDEX review_offers_coach_idx ON review_offers (coach_id, offered_at DESC);

ALTER TABLE review_offers ENABLE ROW LEVEL SECURITY;

CREATE INDEX review_requests_coach_idx ON review_requests (coach_id, status) WHERE coach_id IS NOT NULL;

-- 添削の通知は投稿にひもづかない。
ALTER TABLE notifications
    ALTER COLUMN post_id DROP NOT NULL,
    ADD COLUMN review_request_id uuid REFERENCES review_requests (id) ON DELETE CASCADE,
    ADD CONSTRAINT notifications_target_check CHECK (post_id IS NOT NULL OR review_request_id IS NOT NULL);

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
        'review_expired'
    ));
