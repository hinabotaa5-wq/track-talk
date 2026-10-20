-- name: RecordUpload :exec
-- アップロードの署名を出した相手を記録する。public_id は毎回新しい UUID なので、重なることはない。
INSERT INTO cloudinary_uploads (public_id, owner_id)
VALUES ($1, $2)
ON CONFLICT (public_id) DO NOTHING;

-- name: UploadIssuedTo :one
-- この public_id の署名を、この人に出したか。
SELECT EXISTS (
    SELECT 1 FROM cloudinary_uploads
    WHERE public_id = $1
      AND owner_id = $2
);
