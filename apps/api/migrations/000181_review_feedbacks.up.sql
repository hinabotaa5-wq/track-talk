-- 動画添削が返却されたあと、選手が任意で書ける匿名の感想（本体 #911）。
-- 送信から7日たつまで担当コーチには見せず、見せ始めたときに一度だけ知らせる。
-- どの依頼から送られた感想かは 1添削1件の制限と運営の確認のためだけに持ち、コーチ向けには出さない。
-- 依頼が消えるとき（アカウント削除で依頼ごと消える等）は、他の添削の表と同じく一緒に消す。
CREATE TABLE review_feedbacks (
    id uuid PRIMARY KEY,
    request_id uuid NOT NULL UNIQUE REFERENCES review_requests (id) ON DELETE CASCADE,
    coach_id uuid NOT NULL,
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 500),
    created_at timestamptz NOT NULL DEFAULT now(),
    visible_at timestamptz NOT NULL,
    notified_at timestamptz
);
CREATE INDEX review_feedbacks_coach_visible_idx ON review_feedbacks (coach_id, visible_at);
CREATE INDEX review_feedbacks_unnotified_idx ON review_feedbacks (visible_at) WHERE notified_at IS NULL;
ALTER TABLE review_feedbacks ENABLE ROW LEVEL SECURITY;
