-- 1) golang-migrate が作る schema_migrations だけ RLS が無く、公開の anon キーから書き換えられた。
--    dirty や version を書き換えられると、API は起動時のマイグレーションに失敗して止まる。
ALTER TABLE schema_migrations ENABLE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        REVOKE ALL ON TABLE schema_migrations FROM anon;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        REVOKE ALL ON TABLE schema_migrations FROM authenticated;
    END IF;
END $$;

-- 2) アップロードの署名を出した相手を記録する。画像・動画の public_id は公開 URL や署名付き URL に出るので、
--    ID を知っているだけでは他人の画像・動画を自分の投稿や返答に付けられないようにする。
CREATE TABLE cloudinary_uploads (
    public_id text PRIMARY KEY CHECK (char_length(public_id) BETWEEN 1 AND 255),
    owner_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX cloudinary_uploads_owner_idx ON cloudinary_uploads (owner_id);
ALTER TABLE cloudinary_uploads ENABLE ROW LEVEL SECURITY;

-- すでに使われている ID は、使っている本人のものとして入れておく。
INSERT INTO cloudinary_uploads (public_id, owner_id)
SELECT p.cloudinary_public_id, p.author_id FROM posts p WHERE p.cloudinary_public_id IS NOT NULL
ON CONFLICT DO NOTHING;
INSERT INTO cloudinary_uploads (public_id, owner_id)
SELECT rp.cloudinary_public_id, p.author_id FROM recruitment_photos rp JOIN posts p ON p.id = rp.post_id
ON CONFLICT DO NOTHING;
INSERT INTO cloudinary_uploads (public_id, owner_id)
SELECT photo_public_id, profile_id FROM coach_profiles WHERE photo_public_id IS NOT NULL
ON CONFLICT DO NOTHING;
INSERT INTO cloudinary_uploads (public_id, owner_id)
SELECT video_public_id, requester_id FROM review_requests
ON CONFLICT DO NOTHING;
INSERT INTO cloudinary_uploads (public_id, owner_id)
SELECT demo_video_public_id, coach_id FROM review_responses WHERE demo_video_public_id IS NOT NULL AND coach_id IS NOT NULL
ON CONFLICT DO NOTHING;
INSERT INTO cloudinary_uploads (public_id, owner_id)
SELECT video_public_id, sender_id FROM coaching_messages WHERE video_public_id IS NOT NULL AND sender_id IS NOT NULL
ON CONFLICT DO NOTHING;
