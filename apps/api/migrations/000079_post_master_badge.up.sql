-- 投稿マスターのバッジを投稿の詳細に出すかどうか。本人がマイページで切り替える。既定は出す。
ALTER TABLE profiles
    ADD COLUMN post_master_badge_public boolean NOT NULL DEFAULT true;
