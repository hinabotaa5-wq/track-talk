-- name: CreateCoachingMessage :one
INSERT INTO coaching_messages (id, contract_id, sender_id, sender_role, body, video_public_id, menu)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListCoachingMessages :many
-- やりとり欄。古い順に、最大1000件。
SELECT * FROM coaching_messages
WHERE contract_id = $1
ORDER BY created_at, id
LIMIT 1000;

-- name: CountBlockedContact :exec
-- 連絡先の自動検出で止めた回数を1つ増やす。
INSERT INTO coaching_blocked_counts (contract_id, profile_id, count)
VALUES ($1, $2, 1)
ON CONFLICT (contract_id, profile_id) DO UPDATE
SET count = coaching_blocked_counts.count + 1,
    last_blocked_at = now();

-- name: RateCoachingContract :one
-- 期間が終わったものだけ、利用者が1回だけ★をつけられる。
INSERT INTO coaching_contract_ratings (contract_id, stars)
SELECT ct.id, sqlc.arg(stars)::smallint
FROM coaching_contracts ct
WHERE ct.id = sqlc.arg(contract_id) AND ct.requester_id = sqlc.arg(requester_id) AND ct.status = 'ended'
ON CONFLICT (contract_id) DO NOTHING
RETURNING *;

-- name: GetCoachingContractRating :one
SELECT * FROM coaching_contract_ratings
WHERE contract_id = $1;

-- name: GetCoachingMessageContract :one
-- 通報する1通と、その契約。
SELECT m.id, m.sender_id, m.sender_role, ct.id AS contract_id, ct.requester_id, ct.coach_id, ct.minor
FROM coaching_messages m
JOIN coaching_contracts ct ON ct.id = m.contract_id
WHERE m.id = $1;
