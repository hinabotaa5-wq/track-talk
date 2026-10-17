DROP TABLE review_payments;

DELETE FROM review_requests WHERE status = 'awaiting_payment';

ALTER TABLE review_requests
    DROP CONSTRAINT review_requests_status_check,
    ADD CONSTRAINT review_requests_status_check CHECK (status IN (
        'queued', 'offered', 'in_review', 'returned', 'clarifying', 'expired'
    ));
