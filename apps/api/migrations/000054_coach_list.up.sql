-- コーチ一覧のための列。
ALTER TABLE coach_profiles
    -- 評価が低くて打診の順番から外れたコーチを、運営が戻した時刻。これより前の★は、外す判定に数えない。
    ADD COLUMN rating_reset_at timestamptz,
    -- 顔写真（Cloudinary の公開画像）。コーチ一覧と、返答と一緒に届くプロフィールに出す。
    ADD COLUMN photo_public_id text,
    ADD CONSTRAINT coach_profiles_photo_public_id_check CHECK (photo_public_id IS NULL OR char_length(photo_public_id) BETWEEN 1 AND 255);
