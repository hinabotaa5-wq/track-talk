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

-- name: CountRecentUploads :one
-- この人に、この1日(24時間)に出した署名の数。
SELECT count(*)::int FROM cloudinary_uploads
WHERE owner_id = $1
  AND created_at > now() - interval '1 day';

-- name: LockUploadOwner :exec
-- 同じ人への署名を1つずつにする。1日の上限の確認と記録のあいだに、別の署名を割り込ませない。
SELECT pg_advisory_xact_lock(hashtext('uploads:' || sqlc.arg(owner_id)::uuid::text));
