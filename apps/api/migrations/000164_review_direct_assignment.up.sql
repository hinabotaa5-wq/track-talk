ALTER TABLE review_requests
    ADD COLUMN assigned_at timestamptz,
    ADD COLUMN started_at timestamptz,
    ADD COLUMN timed_out_at timestamptz,
    ADD COLUMN canceled_at timestamptz,
    ADD COLUMN return_reason text CHECK (return_reason IN ('video_unviewable', 'wrong_event', 'medical', 'other')),
    ADD COLUMN reminder_12h_at timestamptz,
    ADD COLUMN reminder_4h_at timestamptz;

-- Existing accepted work may already be underway. Keep its deadline and lock cancellation.
UPDATE review_requests
SET started_at = COALESCE(
    (SELECT max(answered_at) FROM review_offers WHERE request_id = review_requests.id AND result = 'accepted'),
    updated_at
)
WHERE status = 'in_review' OR (status = 'needs_info' AND info_from_status = 'in_review');

-- Move unanswered offers to the coach's queue on the first new tick.
UPDATE review_requests SET status = 'queued', requested_coach_id = COALESCE(requested_coach_id, coach_id), coach_id = NULL
WHERE status = 'offered';
UPDATE review_requests SET info_from_status = 'in_review', assigned_at = now()
WHERE status = 'needs_info' AND info_from_status = 'offered';
UPDATE review_offers SET result = 'expired', answered_at = now() WHERE result IS NULL;

UPDATE push_outbox SET status = 'expired', updated_at = now()
WHERE status = 'pending' AND dedup_key LIKE 'review_offer:%';

CREATE INDEX review_requests_coach_outcomes_idx ON review_requests (coach_id, (COALESCE(timed_out_at, returned_at)) DESC, id DESC)
    WHERE timed_out_at IS NOT NULL OR returned_at IS NOT NULL;
