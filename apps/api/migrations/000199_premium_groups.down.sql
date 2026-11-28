-- 注意：この down は、プレミアムの依頼（動画・グループ・返却を含む）を消してから 000195 の形へ戻す。消した行は戻せない。
-- 動画ファイル（Cloudinary）は消さないので、戻す前に保持期限での削除が済んでいることを確かめること。
DELETE FROM review_responses WHERE good_point IS NULL;

DELETE FROM review_requests WHERE plan = 'premium';

ALTER TABLE review_responses
    DROP CONSTRAINT review_responses_summary_length,
    DROP COLUMN summary;

ALTER TABLE review_response_sections
    DROP CONSTRAINT review_response_sections_group_fkey,
    DROP CONSTRAINT review_response_sections_pkey,
    DROP CONSTRAINT review_response_sections_body_length,
    DROP COLUMN group_position,
    ADD COLUMN slot text NOT NULL,
    ADD PRIMARY KEY (request_id, slot),
    ADD CONSTRAINT review_response_sections_slot_check CHECK (slot IN ('race', 'start', 'second_half', 'practice', 'other')),
    ADD CONSTRAINT review_response_sections_body_length CHECK (char_length(body) BETWEEN 1 AND 1000);

ALTER TABLE review_request_videos
    DROP CONSTRAINT review_request_videos_group_fkey,
    DROP CONSTRAINT review_request_videos_position_range,
    DROP COLUMN group_position,
    ADD COLUMN slot text NOT NULL,
    ADD COLUMN practice_purpose text,
    ADD CONSTRAINT review_request_videos_position_range CHECK (position BETWEEN 1 AND 10),
    ADD CONSTRAINT review_request_videos_slot_check CHECK (slot IN ('race', 'start', 'second_half', 'practice', 'other')),
    ADD CONSTRAINT review_request_videos_race_fields CHECK (slot = 'race' OR (race_time IS NULL AND wind_speed IS NULL AND body_condition IS NULL)),
    ADD CONSTRAINT review_request_videos_practice_fields CHECK (slot = 'practice' OR practice_purpose IS NULL),
    ADD CONSTRAINT review_request_videos_practice_purpose_length CHECK (practice_purpose IS NULL OR char_length(practice_purpose) BETWEEN 1 AND 300);

DROP TABLE review_request_video_groups;
