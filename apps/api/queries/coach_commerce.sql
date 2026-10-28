-- name: GetCoachCommerceSettings :one
SELECT * FROM coach_commerce_settings WHERE coach_id = $1;

-- name: DeclareCoachExempt :one
-- 表記不要の申告。運営が表記要に戻したコーチは申告できない。
INSERT INTO coach_commerce_settings (coach_id, disclosure_required, exempt_checklist_version, exempt_declared_at, updated_at)
VALUES ($1, false, $2, now(), now())
ON CONFLICT (coach_id) DO UPDATE
SET disclosure_required = false, exempt_checklist_version = EXCLUDED.exempt_checklist_version,
    exempt_declared_at = now(), updated_at = now()
WHERE coach_commerce_settings.forced_required_at IS NULL
RETURNING *;

-- name: DeclareCoachRequired :one
INSERT INTO coach_commerce_settings (coach_id, disclosure_required, updated_at)
VALUES ($1, true, now())
ON CONFLICT (coach_id) DO UPDATE
SET disclosure_required = true, exempt_checklist_version = NULL, exempt_declared_at = NULL, updated_at = now()
RETURNING *;

-- name: ForceCoachRequired :one
INSERT INTO coach_commerce_settings (coach_id, disclosure_required, forced_required_at, forced_by, updated_at)
VALUES ($1, true, now(), $2, now())
ON CONFLICT (coach_id) DO UPDATE
SET disclosure_required = true, exempt_checklist_version = NULL, exempt_declared_at = NULL,
    forced_required_at = now(), forced_by = EXCLUDED.forced_by, updated_at = now()
RETURNING *;

-- name: UpsertCoachLegalIdentity :exec
INSERT INTO coach_legal_identities (coach_id, key_id, legal_name_enc, address_enc, phone_enc)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (coach_id) DO UPDATE
SET key_id = EXCLUDED.key_id, legal_name_enc = EXCLUDED.legal_name_enc, address_enc = EXCLUDED.address_enc,
    phone_enc = EXCLUDED.phone_enc, updated_at = now();

-- name: GetCoachLegalIdentity :one
SELECT * FROM coach_legal_identities WHERE coach_id = $1;

-- name: HasCoachLegalIdentity :one
SELECT EXISTS (SELECT 1 FROM coach_legal_identities WHERE coach_id = $1);

-- name: InsertCoachIdentityAccessLog :exec
INSERT INTO coach_identity_access_logs (id, coach_id, admin_id, purpose) VALUES ($1, $2, $3, $4);

-- name: ListCoachIdentityAccessLogs :many
SELECT l.id, l.coach_id, l.admin_id, l.purpose, l.created_at, p.nickname AS admin_nickname
FROM coach_identity_access_logs l LEFT JOIN profiles p ON p.id = l.admin_id
WHERE l.coach_id = $1 ORDER BY l.created_at DESC LIMIT 50;

-- name: EndCoachLegalIdentity :exec
-- コーチをやめた時に、保持期間の起算点を書き留める。最後の取引はこの時点の記録から取る(退会すると記録から本人が外れるため)。
UPDATE coach_legal_identities li
SET coaching_ended_at = now(),
    last_transaction_at = GREATEST(li.last_transaction_at,
        (SELECT max(created_at) FROM coach_earnings WHERE coach_id = li.coach_id),
        (SELECT max(paid_at) FROM coach_payouts WHERE coach_id = li.coach_id),
        (SELECT max(returned_at) FROM review_requests WHERE coach_id = li.coach_id),
        (SELECT max(updated_at) FROM coaching_contracts WHERE coach_id = li.coach_id AND status IN ('ended', 'refunded')),
        (SELECT max(GREATEST(answered_at, clarified_at, closed_at)) FROM coaching_consultations WHERE coach_id = li.coach_id))
WHERE li.coach_id = $1;

-- name: ReopenCoachLegalIdentity :exec
-- もう一度応募したら、やめた日時を空に戻す(活動中は消さない)。
UPDATE coach_legal_identities SET coaching_ended_at = NULL WHERE coach_id = $1;

-- name: DeleteExpiredCoachLegalIdentities :execrows
-- 最後の取引が終わった時とコーチをやめた時の遅いほうが cutoff(今から5年前)より前の登録情報を消す。
-- やめたあとに取引(返金・残りの報酬の支払い)があれば、その日時から数える。活動中・審査待ちのコーチは消さない。
DELETE FROM coach_legal_identities li
WHERE li.coaching_ended_at IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM coach_profiles cp WHERE cp.profile_id = li.coach_id AND cp.status IN ('pending', 'approved'))
  AND GREATEST(li.coaching_ended_at, li.last_transaction_at,
        (SELECT max(created_at) FROM coach_earnings WHERE coach_id = li.coach_id),
        (SELECT max(paid_at) FROM coach_payouts WHERE coach_id = li.coach_id),
        (SELECT max(returned_at) FROM review_requests WHERE coach_id = li.coach_id),
        (SELECT max(updated_at) FROM coaching_contracts WHERE coach_id = li.coach_id AND status IN ('ended', 'refunded')),
        (SELECT max(GREATEST(answered_at, clarified_at, closed_at)) FROM coaching_consultations WHERE coach_id = li.coach_id))
      < sqlc.arg(cutoff)::timestamptz;
