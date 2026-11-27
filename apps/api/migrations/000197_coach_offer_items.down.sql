-- 注意：この down は、添削項目・見本動画の申請と有効な内容、依頼に固定した追加項目、回答の追加項目の本文を消す。消した行は戻せない。
-- 動画ファイル（Cloudinary）は消さない。
DROP TABLE review_response_items;

ALTER TABLE review_requests
    DROP CONSTRAINT review_requests_offer_items_count,
    DROP COLUMN offer_items;

DROP TABLE coach_offers;
DROP TABLE coach_offer_application_videos;
DROP TABLE coach_offer_applications;
