-- name: InsertMinorPurchaseDeclaration :exec
-- 18歳未満の本人が、購入の前に「保護者に許可をもらいました」にチェックしたことを残す。
-- 文面・金額・コーチ・その時の生まれた年月・画面での表示とチェックの時刻もあわせて残す（証跡）。
INSERT INTO minor_purchase_declarations (
    id, minor_id, ref_type, ref_id, text_version, declaration_text, notice_text,
    amount_yen, coach_id, declared_birth_month, client_shown_at, client_checked_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: HasMinorPurchaseDeclaration :one
SELECT EXISTS (
    SELECT 1 FROM minor_purchase_declarations WHERE ref_type = $1 AND ref_id = $2
)::boolean;

-- name: KeepMinorPurchaseDeclarationsOnWithdraw :exec
-- 退会の時に、宣言の記録を残すための印を付ける。退会した日時と、その時点での最後の完了した取引の日時を書き留める
-- （KeepGuardianRecordsOnWithdraw と同じ数え方。退会のあとは支払いの記録から本人が外れるため）。
-- 保管期間を本人の年齢で数えるため、生まれた年月も書き写す（プロフィールはこのあと消える）。
UPDATE minor_purchase_declarations d
SET account_deleted_at = now(),
    birth_month = CASE WHEN d.text_version = 'purchase-notice-2026-11-13' THEN NULL ELSE (SELECT pr.birth_month FROM profiles pr WHERE pr.id = d.minor_id) END,
    last_transaction_at = (
        SELECT max(t) FROM (
            SELECT max(GREATEST(p.captured_at, p.refunded_at)) AS t FROM review_payments p
            WHERE p.payer_id = d.minor_id AND p.status IN ('captured', 'refunded')
            UNION ALL
            SELECT max(GREATEST(c.paid_at, CASE WHEN c.payment_state = 'refunded' THEN c.updated_at END)) FROM coaching_consultations c
            WHERE c.requester_id = d.minor_id AND c.contract_id IS NULL AND c.payment_state <> 'unpaid'
            UNION ALL
            SELECT max(GREATEST(t.paid_at, (SELECT max(r.updated_at) FROM coaching_ticket_refunds r WHERE r.purchase_id = t.id AND r.status = 'succeeded'))) FROM coaching_ticket_purchases t
            WHERE t.owner_id = d.minor_id AND t.status IN ('paid', 'partially_refunded', 'refunded')
              AND d.text_version = 'purchase-notice-2026-11-13'
        ) x
    )
WHERE d.minor_id = sqlc.arg(user_id) AND d.account_deleted_at IS NULL;

-- name: DeleteExpiredMinorPurchaseDeclarations :execrows
-- 保管期間を過ぎた、退会した本人の宣言の記録を消す。退会していない本人の分は対象にしない。
-- 保管期間は、本人が keep_until_age 歳になる月の終わりまで（日本時間。now がそれを過ぎたら消す）。
-- 生まれた年月を書き写す前に退会した分は、従来どおり最後の完了した取引（なければ最後の宣言）が cutoff より前なら消す。
DELETE FROM minor_purchase_declarations d
WHERE d.account_deleted_at IS NOT NULL
  AND CASE WHEN d.birth_month IS NOT NULL THEN
        ((d.birth_month + make_interval(years => sqlc.arg(keep_until_age)::int, months => 1))::timestamp AT TIME ZONE 'Asia/Tokyo')
          <= sqlc.arg(now)::timestamptz
      ELSE
        COALESCE(d.last_transaction_at,
          (SELECT max(x.declared_at) FROM minor_purchase_declarations x WHERE x.minor_id = d.minor_id))
          < sqlc.arg(cutoff)::timestamptz
      END;

-- name: SummarizeMinorSelfPayRefunds :one
-- 18歳未満の本人が自分で払った動画添削・追加相談のうち、[since, until) に返金が済んだものの件数と金額。
-- admin_* は、そのうち管理画面の返金 API（captured_payment_refunds の origin = 'admin'）で返金したもの。
-- 期限切れなどの自動の返金も review_* / consultation_* に含む。
WITH refunds AS (
    SELECT 'review'::text AS source, p.id, p.amount_yen, p.refunded_at AS refunded_at
    FROM review_payments p
    WHERE p.minor AND p.status = 'refunded'
    UNION ALL
    SELECT 'consultation'::text, c.id, c.price_yen, c.updated_at
    FROM coaching_consultations c
    WHERE c.minor AND c.contract_id IS NULL AND c.funding = 'cash' AND c.payment_state = 'refunded'
)
SELECT
    count(*) FILTER (WHERE r.source = 'review')::bigint AS review_count,
    COALESCE(sum(r.amount_yen) FILTER (WHERE r.source = 'review'), 0)::bigint AS review_amount_yen,
    count(*) FILTER (WHERE r.source = 'consultation')::bigint AS consultation_count,
    COALESCE(sum(r.amount_yen) FILTER (WHERE r.source = 'consultation'), 0)::bigint AS consultation_amount_yen,
    count(cr.id)::bigint AS admin_count,
    COALESCE(sum(cr.amount_yen), 0)::bigint AS admin_amount_yen
FROM refunds r
LEFT JOIN captured_payment_refunds cr
    ON cr.source = r.source AND cr.payment_id = r.id AND cr.origin = 'admin' AND cr.status = 'succeeded'
WHERE r.refunded_at >= sqlc.arg(since)::timestamptz AND r.refunded_at < sqlc.arg(until)::timestamptz;
