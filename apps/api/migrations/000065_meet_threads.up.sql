-- 大会当日の実況スレッド。ホームを開いたリクエストのときに、当日の対象大会ごとに1本だけ作る。
-- meet_key は大会の安定した識別子。competitions.id は取り込みのたびに行が消えたり作り直されたり
-- するので使わない(DeleteCompetitionsBySeasonExcept / DeletePastCompetitionSeasons)。meet_threads
-- から competitions へは外部キーを張らない。
-- 実況の投稿は posts.kind = 'meet'、投稿者は下の運営用固定プロフィール。

-- 運営用の固定プロフィール。実況スレッドの投稿者になる。Supabase Auth のユーザーは作らないので
-- ログインできない。ニックネームは公開で固定。is_admin は false(運営の権限は持たない)。
-- ID は apps/api/internal/httpx/meet_threads.go の SystemProfileID と同じ値にする。
INSERT INTO profiles (id, nickname, nickname_public, is_admin)
VALUES ('f47ac10b-58cc-4372-a567-0e02b2c3d479', 'TrackTalk', true, false);

CREATE TABLE meet_threads (
    meet_key text PRIMARY KEY,
    post_id uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE meet_threads ENABLE ROW LEVEL SECURITY;

-- 実況スレッドの投稿の種類 'meet' を許可する(利用者とは別に、運営プロフィールだけが作る)。
ALTER TABLE posts DROP CONSTRAINT posts_kind_check;
ALTER TABLE posts
    ADD CONSTRAINT posts_kind_check CHECK (kind IN ('text', 'youtube', 'instagram', 'video', 'recruitment', 'practice', 'injury', 'meet'));

-- 'meet' は本文だけで、メディア(動画・写真・YouTube・Instagram)を持たない。
ALTER TABLE posts DROP CONSTRAINT posts_kind_fields;
ALTER TABLE posts
    ADD CONSTRAINT posts_kind_fields CHECK (
        (
            kind = 'instagram'
            AND instagram_type IS NOT NULL
            AND instagram_shortcode IS NOT NULL
            AND youtube_video_id IS NULL
            AND youtube_thumbnail_url IS NULL
            AND cloudinary_public_id IS NULL
        )
        OR (
            kind <> 'instagram'
            AND instagram_type IS NULL
            AND instagram_shortcode IS NULL
            AND (
                (
                    kind = 'text'
                    AND youtube_video_id IS NULL
                    AND youtube_thumbnail_url IS NULL
                )
                OR (
                    kind = 'youtube'
                    AND youtube_video_id IS NOT NULL
                    AND youtube_thumbnail_url IS NOT NULL
                    AND cloudinary_public_id IS NULL
                )
                OR (
                    kind = 'video'
                    AND cloudinary_public_id IS NOT NULL
                    AND youtube_video_id IS NULL
                    AND youtube_thumbnail_url IS NULL
                )
                OR (
                    kind IN ('recruitment', 'practice', 'injury', 'meet')
                    AND youtube_video_id IS NULL
                    AND youtube_thumbnail_url IS NULL
                    AND cloudinary_public_id IS NULL
                )
            )
        )
    );
