-- 動画添削の「内容確認のための差し戻し」（運営者決定 2026-11-03）。
-- 依頼の内容が分からないとき、担当のコーチが1依頼につき1回だけ、質問を付けて依頼者に返せる。
-- needs_info: 差し戻し中（依頼者の再提出待ち）。辞退でも終了でもないので、支払いは取り消さない
--   （review_payments を取り消すのは依頼が expired になったときだけ）。
-- 再提出の期限は差し戻しから24時間。過ぎたら tick が expired にし、既存の期限切れと同じく取り消し・返金する。
-- 再提出すると、返却期限は再提出から24時間に数え直す。引受前の差し戻しは、引受期限90分も数え直す。
ALTER TABLE review_requests
    DROP CONSTRAINT review_requests_status_check,
    ADD CONSTRAINT review_requests_status_check CHECK (status IN (
        'awaiting_payment', 'queued', 'offered', 'in_review', 'needs_info', 'returned', 'clarifying', 'expired'
    )),
    -- コーチの質問（どこが・何が分からないか）。
    ADD COLUMN info_request text CHECK (char_length(info_request) <= 1000),
    -- 質問の種類（任意）。focus: 見てほしいポイント、scene: 動画のどの場面か、goal: 目標・現状、mismatch: 動画と説明の食い違い、other: その他。
    ADD COLUMN info_reason text CHECK (info_reason IN ('focus', 'scene', 'goal', 'mismatch', 'other')),
    ADD COLUMN info_requested_at timestamptz,
    -- 差し戻す前の状態。offered（引受前）か in_review（引受後）。再提出したらこの状態に戻す。
    ADD COLUMN info_from_status text CHECK (info_from_status IN ('offered', 'in_review')),
    -- 依頼者の再提出期限。差し戻しから24時間（夜間も止めない）。
    ADD COLUMN resubmit_by timestamptz,
    -- 再提出の前の相談内容。差し戻しの履歴として残す。
    ADD COLUMN original_question text,
    -- 再提出で動画を差し替えたか。差し替える前の動画は残さない（参照が外れたら、使っていない動画として片付く）。
    ADD COLUMN video_replaced boolean NOT NULL DEFAULT false,
    ADD COLUMN resubmitted_at timestamptz,
    ADD CONSTRAINT review_requests_info_check CHECK (
        (info_requested_at IS NULL) = (info_request IS NULL)
        AND (info_requested_at IS NULL) = (info_from_status IS NULL)
        AND (info_requested_at IS NULL) = (resubmit_by IS NULL)
        AND (resubmitted_at IS NULL OR info_requested_at IS NOT NULL)
        AND (status <> 'needs_info' OR (info_requested_at IS NOT NULL AND resubmitted_at IS NULL))
    );

CREATE INDEX review_requests_needs_info_idx ON review_requests (resubmit_by) WHERE status = 'needs_info';

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
        'message'
    ));
