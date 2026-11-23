-- 本体 #981: 取引条件の明示と精算記録（仮払い・確定・帰責判定）の保存期限による削除。日次の storage cleanup から呼ぶ。
-- 対象は契約がすでに消えた（contract_id が NULL の）記録だけ。契約が残っている間は、期限を過ぎていても消さない
-- （精算候補の抽出・保留解除の精算が記録を読むため）。契約終了日の控え（contract_end_on）が無い記録も消さない。
-- 削除できるのは、次の2つの遅い方を過ぎてから（どちらも日本時間の 00:00 から）：
--   税務期限：作成日時を日本時間の暦年で見て、作成した年の8年後の3月16日（翌年3月16日から数えて7年）
--   契約終了基準：契約終了日の7年後の翌日

-- name: DeleteExpiredSupportContractStatements :execrows
DELETE FROM support_contract_statements
WHERE contract_id IS NULL
  AND contract_end_on IS NOT NULL
  AND (make_date(extract(year FROM issued_at AT TIME ZONE 'Asia/Tokyo')::int + 8, 3, 16)::timestamp AT TIME ZONE 'Asia/Tokyo') <= sqlc.arg(now)::timestamptz
  AND ((contract_end_on + interval '7 years' + interval '1 day') AT TIME ZONE 'Asia/Tokyo') <= sqlc.arg(now)::timestamptz;

-- name: DeleteExpiredSupportCoachSettlements :execrows
DELETE FROM support_coach_settlements
WHERE contract_id IS NULL
  AND contract_end_on IS NOT NULL
  AND (make_date(extract(year FROM created_at AT TIME ZONE 'Asia/Tokyo')::int + 8, 3, 16)::timestamp AT TIME ZONE 'Asia/Tokyo') <= sqlc.arg(now)::timestamptz
  AND ((contract_end_on + interval '7 years' + interval '1 day') AT TIME ZONE 'Asia/Tokyo') <= sqlc.arg(now)::timestamptz;

-- name: DeleteExpiredSupportCoachAdvances :execrows
DELETE FROM support_coach_advances
WHERE contract_id IS NULL
  AND contract_end_on IS NOT NULL
  AND (make_date(extract(year FROM created_at AT TIME ZONE 'Asia/Tokyo')::int + 8, 3, 16)::timestamp AT TIME ZONE 'Asia/Tokyo') <= sqlc.arg(now)::timestamptz
  AND ((contract_end_on + interval '7 years' + interval '1 day') AT TIME ZONE 'Asia/Tokyo') <= sqlc.arg(now)::timestamptz;

-- name: DeleteExpiredSupportFaultDeterminations :execrows
DELETE FROM support_fault_determinations
WHERE contract_id IS NULL
  AND contract_end_on IS NOT NULL
  AND (make_date(extract(year FROM notified_at AT TIME ZONE 'Asia/Tokyo')::int + 8, 3, 16)::timestamp AT TIME ZONE 'Asia/Tokyo') <= sqlc.arg(now)::timestamptz
  AND ((contract_end_on + interval '7 years' + interval '1 day') AT TIME ZONE 'Asia/Tokyo') <= sqlc.arg(now)::timestamptz;
