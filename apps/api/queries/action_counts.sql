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
  + (SELECT count(*) FROM notifications n
      WHERE n.recipient_id = sqlc.arg(profile_id)::uuid AND n.coaching_role = 'player' AND n.read_at IS NULL
        AND COALESCE(split_part(n.url, '#', 1), '') NOT IN ('/coach/contracts', '/coach/support-consults')
        AND NOT EXISTS (SELECT 1 FROM review_requests rr
          WHERE rr.requester_id = sqlc.arg(profile_id)::uuid AND rr.status IN ('needs_info', 'awaiting_payment')
            AND (rr.id = n.review_request_id OR split_part(n.url, '#', 1) = '/coaching/requests/' || rr.id::text))
        AND NOT EXISTS (SELECT 1 FROM support_consults sc
          WHERE sc.athlete_id = sqlc.arg(profile_id)::uuid AND sc.status = 'proposed'
            AND split_part(n.url, '#', 1) = '/coaching/support-consults/' || sc.id::text)
        AND NOT EXISTS (SELECT 1 FROM coaching_contracts cc
          WHERE cc.requester_id = sqlc.arg(profile_id)::uuid AND cc.status = 'accepted'
            AND split_part(n.url, '#', 1) = '/coaching/personal/' || cc.id::text))
)::bigint AS player_count;

-- name: CountCoachActionItems :one
-- コーチタブの円ボタン（コーチ）に出す件数。条件は ListCoachReviewing・ListCoachConsultations・
-- コーチ画面の契約タイル（applied）と同じで、無料相談の未返信（requested のみ）だけが新規。
-- clarifying はコーチが確認質問を出して選手の回答待ちなので数えない（#891 補足 2026-11-17）。
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
      WHERE sc.coach_id = sqlc.arg(profile_id)::uuid AND sc.status = 'requested')
  + (SELECT count(*) FROM notifications n
      WHERE n.recipient_id = sqlc.arg(profile_id)::uuid AND n.coaching_role = 'coach' AND n.read_at IS NULL
        AND COALESCE(split_part(n.url, '#', 1), '') NOT IN ('/coach/contracts', '/coach/support-consults')
        AND NOT EXISTS (SELECT 1 FROM review_requests rr
          WHERE rr.coach_id = sqlc.arg(profile_id)::uuid AND rr.status IN ('in_review', 'needs_info')
            AND (rr.id = n.review_request_id OR split_part(n.url, '#', 1) = '/coach/requests/' || rr.id::text))
        AND NOT EXISTS (SELECT 1 FROM coaching_contracts cc
          WHERE cc.coach_id = sqlc.arg(profile_id)::uuid AND cc.status = 'applied'
            AND split_part(n.url, '#', 1) = '/coaching/personal/' || cc.id::text)
        AND NOT EXISTS (SELECT 1 FROM coaching_consultations c
          WHERE c.coach_id = sqlc.arg(profile_id)::uuid AND c.purged_at IS NULL
            AND (c.status IN ('offered', 'in_review')
              OR (c.status = 'answered' AND c.clarify_requested_at IS NOT NULL AND c.clarified_at IS NULL))
            AND split_part(n.url, '#', 1) = '/coach/requests/' || c.review_request_id::text))
)::bigint AS coach_count;
