DROP INDEX inquiries_responded_idx;

ALTER TABLE inquiries
    DROP COLUMN responder_id,
    DROP COLUMN response_body,
    DROP COLUMN responded_at;
