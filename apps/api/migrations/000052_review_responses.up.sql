-- 添削の返答・補足・評価。流れは online-coaching/design-review.md の「2-3」「2-4」のとおり。

ALTER TABLE review_requests
    ADD COLUMN returned_at timestamptz;

-- コーチの返答と、「わからなかった」に対する補足（1回だけ）。
CREATE TABLE review_responses (
    request_id uuid PRIMARY KEY REFERENCES review_requests (id) ON DELETE CASCADE,
    coach_id uuid REFERENCES profiles (id) ON DELETE SET NULL,
    good_point text NOT NULL,
    fix_points text[] NOT NULL,
    drill text NOT NULL,
    demo_video_public_id text UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    clarify_reason text,
    clarify_requested_at timestamptz,
    clarification text,
    clarified_at timestamptz,
    -- 7日後の「役に立ちましたか」を送った時刻。
    helpful_asked_at timestamptz,
    -- 通報を受けて運営が隠した時刻。
    hidden_at timestamptz,
    CHECK (char_length(good_point) BETWEEN 1 AND 300),
    CHECK (cardinality(fix_points) BETWEEN 1 AND 2),
    CHECK (char_length(drill) BETWEEN 1 AND 300),
    CHECK (clarify_reason IN ('term', 'drill', 'fix', 'other')),
    CHECK ((clarify_reason IS NULL) = (clarify_requested_at IS NULL)),
    CHECK (clarification IS NULL OR char_length(clarification) BETWEEN 1 AND 300),
    CHECK ((clarification IS NULL) = (clarified_at IS NULL)),
    CHECK (clarification IS NULL OR clarify_requested_at IS NOT NULL)
);

CREATE INDEX review_responses_coach_idx ON review_responses (coach_id, created_at);

ALTER TABLE review_responses ENABLE ROW LEVEL SECURITY;

-- 利用者の評価。★はいつでも、「役に立ったか」は7日後に聞く。
CREATE TABLE review_ratings (
    request_id uuid PRIMARY KEY REFERENCES review_requests (id) ON DELETE CASCADE,
    stars smallint,
    rated_at timestamptz,
    helpful text,
    helpful_at timestamptz,
    CHECK (stars BETWEEN 1 AND 5),
    CHECK ((stars IS NULL) = (rated_at IS NULL)),
    CHECK (helpful IN ('yes', 'some', 'no')),
    CHECK ((helpful IS NULL) = (helpful_at IS NULL))
);

ALTER TABLE review_ratings ENABLE ROW LEVEL SECURITY;

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
        'review_helpful_ask'
    ));

ALTER TABLE reports DROP CONSTRAINT reports_target_type_check;
ALTER TABLE reports ADD CONSTRAINT reports_target_type_check CHECK (target_type IN ('post', 'comment', 'injury_note', 'review_response'));
