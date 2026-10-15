-- name: UpsertPushSubscription :one
-- 同じ端末を別のアカウントで登録し直したら、新しいアカウントに付け替える。
INSERT INTO push_subscriptions (id, profile_id, endpoint, p256dh, auth)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (endpoint) DO UPDATE
SET profile_id = EXCLUDED.profile_id,
    p256dh = EXCLUDED.p256dh,
    auth = EXCLUDED.auth,
    created_at = now()
RETURNING *;

-- name: DeletePushSubscription :execrows
DELETE FROM push_subscriptions
WHERE profile_id = $1
  AND endpoint = $2;

-- name: DeletePushSubscriptionByEndpoint :exec
DELETE FROM push_subscriptions
WHERE endpoint = $1;

-- name: ListPushSubscriptions :many
SELECT * FROM push_subscriptions
WHERE profile_id = $1
ORDER BY created_at DESC;

-- name: CountPushSubscriptions :one
SELECT count(*) FROM push_subscriptions
WHERE profile_id = $1;
