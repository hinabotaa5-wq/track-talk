-- 動画ファイルの保持（運営者決定 2026-11-10、本体 #506）。
-- 動画添削：スレッドが終わってから7日で、依頼の動画とコーチの見本動画を消す。
-- サポートプラン：契約が終わってから30日で、やりとり欄の動画を消す。
-- 追加相談：添削のものは相談が終わってから7日、契約のものは契約と相談の遅い方の終わりから30日で消す。
-- 通報・紛争の対応中（未対応の通報、異議の申立ての再審査待ち、運営の保全ホールド）は消さない。
-- 文章・メニュー・取引の記録はここでは消さない（保持は従来どおり）。

-- name: ListReviewVideosToPurge :many
-- スレッドの終わりは、返却して「わからなかった」の確認の往復が終わった時。確認を頼めるのは返却から7日なので、
-- 頼まれなかったときは返却から7日（頼める期間が閉じた時）。返却されずに終わった依頼は、終わった時。
-- 添削のあとの追加相談が進んでいるあいだは、コーチが元の動画を見るので消さない。
SELECT r.id FROM review_requests r
LEFT JOIN review_responses rs ON rs.request_id = r.id
WHERE r.videos_purged_at IS NULL AND r.retention_hold_at IS NULL
  AND CASE
      WHEN r.status = 'returned' AND rs.clarify_requested_at IS NULL THEN r.returned_at + interval '7 days'
      WHEN r.status = 'returned' THEN rs.clarified_at
      WHEN r.status = 'expired' THEN r.updated_at
  END <= now() - interval '7 days'
  AND NOT EXISTS (SELECT 1 FROM reports rp WHERE rp.target_type = 'review_response' AND rp.status = 'open' AND rp.target_id = r.id)
  AND NOT EXISTS (SELECT 1 FROM reports rp JOIN coaching_consultations c ON c.id = rp.target_id
                  WHERE rp.target_type = 'coaching_consultation' AND rp.status = 'open' AND c.review_request_id = r.id)
  AND NOT EXISTS (SELECT 1 FROM coaching_consultations c WHERE c.review_request_id = r.id AND c.purged_at IS NULL
                  AND (c.status IN ('awaiting_payment', 'offered', 'in_review') OR (c.status = 'draft' AND c.expires_at > now())
                       OR (c.clarify_requested_at IS NOT NULL AND c.clarified_at IS NULL)))
ORDER BY r.updated_at
LIMIT 20;

-- name: ListReviewVideoIDs :many
SELECT r.video_public_id::text FROM review_requests r WHERE r.id = sqlc.arg(id)
UNION ALL
SELECT rs.demo_video_public_id::text FROM review_responses rs WHERE rs.request_id = sqlc.arg(id) AND rs.demo_video_public_id IS NOT NULL;

-- name: MarkReviewVideosPurged :exec
-- updated_at は変えない（返却されずに終わった依頼の、終わった時として使う）。
UPDATE review_requests SET videos_purged_at = now() WHERE id = $1 AND videos_purged_at IS NULL;

-- name: ContractRetentionHeld :one
-- 契約のやりとりを保全しているか。未対応の通報（やりとり欄・追加相談）、異議の申立ての再審査待ち、運営の保全ホールド。
SELECT (ct.retention_hold_at IS NOT NULL
    OR EXISTS (SELECT 1 FROM reports rp JOIN coaching_messages m ON m.id = rp.target_id
               WHERE rp.target_type = 'coaching_message' AND rp.status = 'open' AND m.contract_id = ct.id)
    OR EXISTS (SELECT 1 FROM reports rp JOIN coaching_consultations c ON c.id = rp.target_id
               WHERE rp.target_type = 'coaching_consultation' AND rp.status = 'open' AND c.contract_id = ct.id)
    OR EXISTS (SELECT 1 FROM support_fault_determinations f
               WHERE f.contract_id = ct.id AND f.objected_at IS NOT NULL AND f.reviewed_at IS NULL))::bool AS held
FROM coaching_contracts ct WHERE ct.id = $1;

-- name: ListContractVideosToPurge :many
-- 契約が終わって（満了・途中終了とも）30日たち、やりとり欄の動画を消す契約。
SELECT ct.id FROM coaching_contracts ct
WHERE ct.status IN ('ended', 'refunded') AND ct.ends_at <= now() - interval '30 days'
  AND ct.videos_purged_at IS NULL AND ct.messages_purged_at IS NULL AND ct.retention_hold_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM reports rp JOIN coaching_messages m ON m.id = rp.target_id
                  WHERE rp.target_type = 'coaching_message' AND rp.status = 'open' AND m.contract_id = ct.id)
  AND NOT EXISTS (SELECT 1 FROM reports rp JOIN coaching_consultations c ON c.id = rp.target_id
                  WHERE rp.target_type = 'coaching_consultation' AND rp.status = 'open' AND c.contract_id = ct.id)
  AND NOT EXISTS (SELECT 1 FROM support_fault_determinations f
                  WHERE f.contract_id = ct.id AND f.objected_at IS NOT NULL AND f.reviewed_at IS NULL)
ORDER BY ct.ends_at
LIMIT 20;

-- name: MarkContractVideosPurged :exec
UPDATE coaching_contracts SET videos_purged_at = now() WHERE id = $1 AND videos_purged_at IS NULL;

-- name: ListConsultationVideosToPurge :many
-- 追加相談の動画（質問と回答）。終わり（回答・辞退・期限切れ・取り下げのうち最後の時）から、
-- 添削のものは7日、契約のものは契約の終わりとの遅い方から30日。元の添削・契約を保全しているあいだは消さない。
SELECT c.id FROM coaching_consultations c
LEFT JOIN review_requests r ON r.id = c.review_request_id
LEFT JOIN coaching_contracts ct ON ct.id = c.contract_id
WHERE c.videos_purged_at IS NULL AND c.purged_at IS NULL
  AND (c.video_public_id IS NOT NULL OR c.answer_video_public_id IS NOT NULL)
  AND c.status IN ('answered', 'declined', 'expired', 'cancelled')
  AND NOT (c.clarify_requested_at IS NOT NULL AND c.clarified_at IS NULL)
  AND GREATEST(c.closed_at, c.answered_at, c.clarified_at) IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM reports rp WHERE rp.target_type = 'coaching_consultation' AND rp.status = 'open'
                  AND rp.target_id IN (SELECT s.id FROM coaching_consultations s
                                       WHERE s.review_request_id = c.review_request_id OR s.contract_id = c.contract_id))
  AND (
      (r.id IS NOT NULL AND r.retention_hold_at IS NULL
       AND GREATEST(c.closed_at, c.answered_at, c.clarified_at) <= now() - interval '7 days'
       AND NOT EXISTS (SELECT 1 FROM reports rp WHERE rp.target_type = 'review_response' AND rp.status = 'open' AND rp.target_id = r.id))
   OR (ct.id IS NOT NULL AND ct.status IN ('ended', 'refunded') AND ct.retention_hold_at IS NULL
       AND GREATEST(ct.ends_at, c.closed_at, c.answered_at, c.clarified_at) <= now() - interval '30 days'
       AND NOT EXISTS (SELECT 1 FROM reports rp JOIN coaching_messages m ON m.id = rp.target_id
                       WHERE rp.target_type = 'coaching_message' AND rp.status = 'open' AND m.contract_id = ct.id)
       AND NOT EXISTS (SELECT 1 FROM support_fault_determinations f
                       WHERE f.contract_id = ct.id AND f.objected_at IS NOT NULL AND f.reviewed_at IS NULL)))
ORDER BY c.created_at
LIMIT 20;

-- name: MarkConsultationVideosPurged :exec
UPDATE coaching_consultations SET videos_purged_at = now() WHERE id = $1 AND videos_purged_at IS NULL;

-- name: SetReviewRetentionHold :one
-- 運営の保全ホールドを立てる・外す。
UPDATE review_requests SET retention_hold_at = CASE WHEN sqlc.arg(hold)::bool THEN COALESCE(retention_hold_at, now()) END
WHERE id = sqlc.arg(id)
RETURNING id, retention_hold_at, videos_purged_at;

-- name: SetContractRetentionHold :one
UPDATE coaching_contracts SET retention_hold_at = CASE WHEN sqlc.arg(hold)::bool THEN COALESCE(retention_hold_at, now()) END
WHERE id = sqlc.arg(id)
RETURNING id, retention_hold_at, videos_purged_at;

-- name: ClaimContractCloseNotices :many
-- 終わる3日前になった期間中の契約に、終わると見られなくなることとダウンロードの案内を1回だけ送る。
UPDATE coaching_contracts SET close_notice_at = now()
WHERE status = 'active' AND close_notice_at IS NULL AND ends_at > now() AND ends_at <= now() + interval '3 days'
RETURNING id, requester_id, coach_id, plan_title, ends_at;
