-- 動画添削の「引受後のコーチ都合の辞退」（運営者決定 2026-11-03）。
-- 担当のコーチは、引き受けた依頼を返却するまで辞退できる（添削中、または引受後に差し戻して再提出を待っている間）。
-- 辞退した依頼はその場で expired にする。支払いの取り消し・返金とチケットの返却は、既存の期限切れと同じく tick が行う。
-- 別のコーチには回さない。ペナルティや手数料はない。誰が・いつ・どの状態から辞退したかを依頼に残す。
ALTER TABLE review_requests
    ADD COLUMN withdrawn_at timestamptz,
    -- 辞退したコーチ。coach_id もそのまま残す（添削中の期限切れと同じ）。
    ADD COLUMN withdrawn_by uuid REFERENCES profiles (id) ON DELETE SET NULL,
    -- 辞退した時の状態。in_review（添削中）か needs_info（引受後の差し戻し中）。
    ADD COLUMN withdrawn_from_status text CHECK (withdrawn_from_status IN ('in_review', 'needs_info')),
    -- コーチの任意の短い理由。依頼者に見せる。
    ADD COLUMN withdraw_reason text CHECK (char_length(withdraw_reason) <= 200),
    ADD CONSTRAINT review_requests_withdraw_check CHECK (
        (withdrawn_at IS NULL) = (withdrawn_from_status IS NULL)
        AND (withdraw_reason IS NULL OR withdrawn_at IS NOT NULL)
        AND (withdrawn_at IS NULL OR status = 'expired')
    );

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
        'review_info_request',
        'review_resubmitted',
        'review_withdrawn',
        'message'
    ));
