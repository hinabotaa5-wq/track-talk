-- Rollback requires every review request to have a video_public_id. NULL rows prevent SET NOT NULL.
ALTER TABLE review_requests ALTER COLUMN video_public_id SET NOT NULL;
