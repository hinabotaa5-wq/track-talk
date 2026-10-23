-- 回収が決まったファイルを、古い送信画面から投稿に付け直せないようにする。
-- 回収側と書き込み側で同じロックを使い、使用確認と削除の間の競合を防ぐ。
CREATE TABLE cloudinary_retirements (
    public_id text PRIMARY KEY,
    retired_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE cloudinary_retirements ENABLE ROW LEVEL SECURITY;

CREATE FUNCTION guard_retired_cloudinary_media() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE media_id text;
BEGIN
    media_id := to_jsonb(NEW)->>TG_ARGV[0];
    IF media_id IS NULL OR media_id = '' THEN RETURN NEW; END IF;
    IF TG_OP = 'UPDATE' AND media_id IS NOT DISTINCT FROM (to_jsonb(OLD)->>TG_ARGV[0]) THEN RETURN NEW; END IF;
    IF TG_ARGV[0] = 'image_url' THEN
        media_id := substring(replace(media_id, '%2F', '/') from '(track-talk(-coaching)?/[0-9a-fA-F-]{36})');
        IF media_id IS NULL THEN RETURN NEW; END IF;
    END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended('media-retire:' || media_id, 0));
    IF EXISTS (SELECT 1 FROM cloudinary_retirements WHERE public_id = media_id) THEN
        RAISE EXCEPTION 'media has been retired' USING ERRCODE = '23514', CONSTRAINT = 'cloudinary_media_retired';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER guard_retired_media BEFORE INSERT OR UPDATE OF cloudinary_public_id ON posts FOR EACH ROW EXECUTE FUNCTION guard_retired_cloudinary_media('cloudinary_public_id');
CREATE TRIGGER guard_retired_media BEFORE INSERT OR UPDATE OF cloudinary_public_id ON recruitment_photos FOR EACH ROW EXECUTE FUNCTION guard_retired_cloudinary_media('cloudinary_public_id');
CREATE TRIGGER guard_retired_media BEFORE INSERT OR UPDATE OF photo_public_id ON coach_profiles FOR EACH ROW EXECUTE FUNCTION guard_retired_cloudinary_media('photo_public_id');
CREATE TRIGGER guard_retired_media BEFORE INSERT OR UPDATE OF video_public_id ON review_requests FOR EACH ROW EXECUTE FUNCTION guard_retired_cloudinary_media('video_public_id');
CREATE TRIGGER guard_retired_media BEFORE INSERT OR UPDATE OF demo_video_public_id ON review_responses FOR EACH ROW EXECUTE FUNCTION guard_retired_cloudinary_media('demo_video_public_id');
CREATE TRIGGER guard_retired_media BEFORE INSERT OR UPDATE OF video_public_id ON coaching_messages FOR EACH ROW EXECUTE FUNCTION guard_retired_cloudinary_media('video_public_id');
CREATE TRIGGER guard_retired_media BEFORE INSERT OR UPDATE OF video_public_id ON coaching_consultations FOR EACH ROW EXECUTE FUNCTION guard_retired_cloudinary_media('video_public_id');
CREATE TRIGGER guard_retired_answer_media BEFORE INSERT OR UPDATE OF answer_video_public_id ON coaching_consultations FOR EACH ROW EXECUTE FUNCTION guard_retired_cloudinary_media('answer_video_public_id');
CREATE TRIGGER guard_retired_media BEFORE INSERT OR UPDATE OF image_url ON products FOR EACH ROW EXECUTE FUNCTION guard_retired_cloudinary_media('image_url');
CREATE TRIGGER guard_retired_media BEFORE INSERT OR UPDATE OF image_url ON major_hosts FOR EACH ROW EXECUTE FUNCTION guard_retired_cloudinary_media('image_url');
