-- name: CountPlayerActionItems :one
-- コーチタブの円ボタン（選手）に出す「自分が対応しないと進まない用事」の件数（本体 #891）。
-- 1回プランの差し戻し・支払い待ち、無料相談の提案への返事待ち、継続プランの開始手続き待ちの合計。
SELECT (
    (SELECT count(*) FROM review_requests rr
      WHERE rr.requester_id = sqlc.arg(profile_id)::uuid AND rr.status IN ('needs_info', 'awaiting_payment'))
  + (SELECT count(*) FROM support_consults sc
      WHERE sc.athlete_id = sqlc.arg(profile_id)::uuid AND sc.status = 'proposed')
  + (SELECT count(*) FROM coaching_contracts cc
      WHERE cc.requester_id = sqlc.arg(profile_id)::uuid AND cc.status = 'accepted')
)::bigint AS player_count;

-- name: CountCoachActionItems :one
-- コーチタブの円ボタン（コーチ）に出す件数。条件は ListCoachReviewing・ListCoachConsultations・
-- コーチ画面の契約タイル（applied）と同じで、無料相談の未返信（requested・clarifying）だけが新規。
SELECT (
    (SELECT count(*) FROM review_requests rr
      WHERE rr.coach_id = sqlc.arg(profile_id)::uuid AND rr.status IN ('in_review', 'needs_info'))
  + (SELECT count(*) FROM coaching_consultations c
      WHERE c.coach_id = sqlc.arg(profile_id)::uuid AND c.purged_at IS NULL
        AND (c.status IN ('offered', 'in_review')
             OR (c.status = 'answered' AND c.clarify_requested_at IS NOT NULL AND c.clarified_at IS NULL)))
  + (SELECT count(*) FROM coaching_contracts cc
      WHERE cc.coach_id = sqlc.arg(profile_id)::uuid AND cc.status = 'applied')
  + (SELECT count(*) FROM support_consults sc
      WHERE sc.coach_id = sqlc.arg(profile_id)::uuid AND sc.status IN ('requested', 'clarifying'))
)::bigint AS coach_count;
