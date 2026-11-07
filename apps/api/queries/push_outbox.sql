-- 送るべき Push の待ち行列（本体 #421 F04・F11）。一時障害のあとに送り直す。
-- 宛先単位ではなく依頼・配信単位で1行作り、送るたびにいまの購読と設定を確かめる。

-- name: EnqueuePushOutbox :exec
-- 同じ出来事は1行だけ。二重に入れても増えない。
INSERT INTO push_outbox (id, profile_id, title, body, url, tag, category, ttl_seconds, dedup_key, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (dedup_key) DO NOTHING;

-- name: ExpirePushOutbox :execrows
-- 有効期限を過ぎた待ちを終わりにする。期限切れの打診などは送らない。
UPDATE push_outbox
SET status = 'expired',
    last_result = 'expired',
    updated_at = now()
WHERE status = 'pending'
  AND expires_at <= now();

-- name: PurgePushOutbox :exec
-- 終わった行のうち古いものを消す。待ち行列が際限なく増えないようにする。
DELETE FROM push_outbox
WHERE status <> 'pending'
  AND updated_at < now() - interval '30 days';

-- name: ClaimPendingPushOutbox :many
-- 送りどきの待ちを古い順に取る。複数の処理が重なっても同じ行を二重に送らない。
SELECT id, profile_id, title, body, url, tag, category, ttl_seconds, dedup_key, attempts, expires_at
FROM push_outbox
WHERE status = 'pending'
  AND next_attempt_at <= now()
  AND expires_at > now()
ORDER BY next_attempt_at
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkPushOutboxSent :exec
UPDATE push_outbox
SET status = 'sent',
    attempts = attempts + 1,
    last_result = 'sent',
    updated_at = now(),
    sent_at = now()
WHERE id = $1
  AND status = 'pending';

-- name: MarkPushOutboxFailed :exec
-- 送れなかった待ちを、待ち時間をおいて送り直す。間隔は回数で延ばし、最長10分。
UPDATE push_outbox
SET attempts = attempts + 1,
    last_result = sqlc.arg('result'),
    next_attempt_at = now() + make_interval(mins => CASE WHEN attempts < 1 THEN 1 WHEN attempts < 2 THEN 2 WHEN attempts < 3 THEN 4 ELSE 10 END),
    updated_at = now()
WHERE id = sqlc.arg('id')
  AND status = 'pending';

-- name: MarkPushOutboxDiscarded :exec
-- 設定で送らないことになった待ちは、送り直さず終わりにする（途中で設定を切った人）。
UPDATE push_outbox
SET status = 'discarded',
    last_result = sqlc.arg('result'),
    updated_at = now()
WHERE id = sqlc.arg('id')
  AND status = 'pending';

-- name: CountPendingPushOutbox :one
SELECT count(*) FROM push_outbox
WHERE status = 'pending';

-- name: CountPushOutboxByPrefix :one
-- 一斉配信の進み具合。送り待ちと送り済みを数え、再開の要否と送った人数に使う。
SELECT
    (SELECT count(*) FROM push_outbox WHERE status = 'pending' AND push_outbox.dedup_key LIKE $1) AS pending,
    (SELECT count(*) FROM push_outbox WHERE status = 'sent' AND push_outbox.dedup_key LIKE $1) AS sent;
