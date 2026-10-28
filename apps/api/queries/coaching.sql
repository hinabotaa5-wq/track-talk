-- name: SetBirthMonth :one
-- 生まれた年月は一度だけ登録できる。18歳未満が大人に書き換えるのを防ぐため、変更は問い合わせで受ける。
UPDATE profiles
SET birth_month = $2,
    updated_at = now()
WHERE id = $1
  AND birth_month IS NULL
RETURNING *;

-- name: GetGuardianConsentByMinor :one
SELECT * FROM guardian_consents
WHERE minor_id = $1;

-- name: GetGuardianConsentByToken :one
SELECT * FROM guardian_consents
WHERE token_hash = $1
  AND link_issued_at > now() - interval '30 days'
  AND account_deleted_at IS NULL;

-- name: IssueGuardianLink :one
-- リンクを出し直すと前のリンクは使えなくなる。リンクは出してから30日で使えなくなる。同意の状態はそのまま残す。
-- 1日（日本時間）に送れるのは daily_limit 回まで。保護者が一度答えたあとは、送り先のメールアドレスを変えられない。
-- どちらかに当たると行を返さない。
INSERT INTO guardian_consents (minor_id, token_hash, guardian_email, link_sent_on, link_sends)
VALUES ($1, $2, $3, (now() AT TIME ZONE 'Asia/Tokyo')::date, 1)
ON CONFLICT (minor_id) DO UPDATE
SET token_hash = EXCLUDED.token_hash,
    guardian_email = EXCLUDED.guardian_email,
    link_sends = CASE WHEN guardian_consents.link_sent_on = EXCLUDED.link_sent_on THEN guardian_consents.link_sends + 1 ELSE 1 END,
    link_sent_on = EXCLUDED.link_sent_on,
    link_issued_at = now(),
    updated_at = now()
WHERE (guardian_consents.link_sent_on IS DISTINCT FROM EXCLUDED.link_sent_on OR guardian_consents.link_sends < sqlc.arg(daily_limit)::integer)
  AND (guardian_consents.status = 'pending' OR lower(guardian_consents.guardian_email) = lower(EXCLUDED.guardian_email))
RETURNING *;

-- name: ConsentGuardian :one
-- 同意は、本人が送った同意用のリンクからだけ、1つのリンクにつき1回だけできる（同意に使ったリンクは consent_token_hash に残る）。
-- 同意し直すときは、本人に新しいリンクを送ってもらう。
UPDATE guardian_consents
SET status = 'consented',
    guardian_name = sqlc.arg(guardian_name),
    relationship = sqlc.arg(relationship),
    monthly_limit_yen = sqlc.arg(monthly_limit_yen),
    self_pay_limit_yen = sqlc.arg(self_pay_limit_yen),
    allow_publish = sqlc.arg(allow_publish),
    consented_at = now(),
    consent_ip = sqlc.arg(consent_ip),
    terms_version = sqlc.arg(terms_version),
    consent_token_hash = token_hash,
    declined_at = NULL,
    revoked_at = NULL,
    updated_at = now()
WHERE token_hash = sqlc.arg(token_hash)
  AND status IN ('pending', 'declined', 'revoked')
  AND consent_token_hash IS DISTINCT FROM token_hash
  AND account_deleted_at IS NULL
RETURNING *;

-- name: UpdateGuardianSettings :one
UPDATE guardian_consents
SET monthly_limit_yen = $2,
    allow_publish = $3,
    self_pay_limit_yen = $4,
    updated_at = now()
WHERE minor_id = $1
  AND status = 'consented'
RETURNING *;

-- name: DeclineGuardian :one
UPDATE guardian_consents
SET status = 'declined',
    declined_at = now(),
    updated_at = now()
WHERE token_hash = $1
  AND status IN ('pending', 'declined')
  AND account_deleted_at IS NULL
RETURNING *;

-- name: RevokeGuardian :one
UPDATE guardian_consents
SET status = 'revoked',
    revoked_at = now(),
    updated_at = now()
WHERE minor_id = $1
  AND status = 'consented'
RETURNING *;

-- name: GetGuardianConsentByManageLink :one
-- 上限の変更・撤回のためのリンク。期限を過ぎたものは使えない。
SELECT gc.* FROM guardian_manage_links l
JOIN guardian_consents gc ON gc.minor_id = l.minor_id
WHERE l.token_hash = $1
  AND l.expires_at > now()
  AND gc.account_deleted_at IS NULL;

-- name: CreateGuardianManageLink :exec
INSERT INTO guardian_manage_links (token_hash, minor_id, expires_at)
VALUES ($1, $2, $3);

-- name: InsertGuardianConsentEvent :exec
INSERT INTO guardian_consent_events (minor_id, event, guardian_email, guardian_name, relationship, self_pay_limit_yen, previous_self_pay_limit_yen, terms_version, ip)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ListGuardianConsentEvents :many
SELECT * FROM guardian_consent_events
WHERE minor_id = $1
ORDER BY id;

-- name: ClaimGuardianLimitRequest :one
-- 上限変更のお願いは、同じ子について12時間に1回まで。送ってよいときだけ行を返す。
UPDATE guardian_consents
SET limit_request_sent_at = now()
WHERE minor_id = $1
  AND status = 'consented'
  AND guardian_email IS NOT NULL
  AND (limit_request_sent_at IS NULL OR limit_request_sent_at < now() - interval '12 hours')
RETURNING *;

-- name: ReleaseGuardianLimitRequest :exec
-- 上限変更のお願いを送れなかったときは、次の購入の時にまた送れるようにする。
UPDATE guardian_consents
SET limit_request_sent_at = NULL
WHERE minor_id = $1;

-- name: SumMinorSelfPaidSpend :one
-- 18歳未満が自分で払う動画添削と追加相談（添削のあとの相談）の、今月（日本時間）の合計。
-- 確定した支払い（カードの仮押さえを含む）と、まだ閉じていない支払い画面を数える。返金・取り消しは数えない。
-- 保護者が払うパーソナル指導・サポートプランとその追加相談は含めない。
SELECT COALESCE(sum(x.amount), 0)::bigint FROM (
    SELECT amount_yen::bigint AS amount FROM review_payments
    WHERE payer_id = sqlc.arg(minor_id)::uuid
      AND request_id IS DISTINCT FROM sqlc.arg(exclude_review_id)::uuid
      AND created_at >= date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo') AT TIME ZONE 'Asia/Tokyo'
      AND (status IN ('authorized', 'captured') OR (status = 'pending' AND session_expires_at > now()))
    UNION ALL
    SELECT c.price_yen::bigint FROM coaching_consultations c
    WHERE c.requester_id = sqlc.arg(minor_id)::uuid
      AND c.funding = 'cash'
      AND c.contract_id IS NULL
      AND c.id IS DISTINCT FROM sqlc.arg(exclude_consultation_id)::uuid
      AND COALESCE(c.paid_at, c.updated_at) >= date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo') AT TIME ZONE 'Asia/Tokyo'
      AND (c.payment_state IN ('paid', 'refund_pending')
           OR (c.status = 'awaiting_payment' AND c.expires_at > now() AND (c.session_expires_at IS NULL OR c.session_expires_at > now())))
) x;

-- name: CreateGuardianNotice :one
-- 同じ購入の通知はすでにあれば作らない（行を返さない）。
INSERT INTO guardian_notices (id, minor_id, kind, ref_type, ref_id, guardian_email, amount_yen, coach_name, purchased_at, status, payment_intent_id, sent_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, CASE WHEN $10 = 'sent' THEN now() END)
ON CONFLICT (kind, ref_type, ref_id) DO NOTHING
RETURNING *;

-- name: MarkGuardianNotice :exec
UPDATE guardian_notices
SET status = $2,
    last_error = $3,
    attempts = attempts + 1,
    sent_at = CASE WHEN $2 = 'sent' THEN now() ELSE sent_at END,
    updated_at = now()
WHERE id = $1;

-- name: ListGuardianNoticesToSend :many
-- 送れていない購入の通知。作ってから1分たったもの（作った直後は、作った側が送っている）を、5回まで送り直す。
-- 退会した本人の分は送らない（変更・撤回のリンクが使えないため）。
SELECT n.* FROM guardian_notices n
WHERE n.status <> 'sent'
  AND n.kind = 'purchase'
  AND n.attempts < 5
  AND n.updated_at < now() - interval '1 minute'
  AND NOT EXISTS (SELECT 1 FROM guardian_consents gc WHERE gc.minor_id = n.minor_id AND gc.account_deleted_at IS NOT NULL)
ORDER BY n.updated_at
LIMIT 50;

-- name: ListGuardianNotices :many
SELECT * FROM guardian_notices
WHERE minor_id = $1
ORDER BY created_at, id;

-- name: KeepGuardianRecordsOnWithdraw :exec
-- 退会の時に、保護者同意の証跡を残すための印を付ける。退会した日時と、その時点での最後の完了した取引の日時を書き留める
-- （退会のあとは支払いの記録から本人が外れるため）。取引は、本人が払った動画添削（確定・返金）と追加相談（支払い・返金）、
-- 保護者に通知した購入。変更・撤回用のリンクは消し、同意用のリンクも使えなくする（GetGuardianConsentByToken などで見る）。
WITH marked AS (
    UPDATE guardian_consents gc
    SET account_deleted_at = now(),
        last_transaction_at = (
            SELECT max(t) FROM (
                SELECT max(GREATEST(p.captured_at, p.refunded_at)) AS t FROM review_payments p
                WHERE p.payer_id = gc.minor_id AND p.status IN ('captured', 'refunded')
                UNION ALL
                SELECT max(GREATEST(c.paid_at, CASE WHEN c.payment_state = 'refunded' THEN c.updated_at END)) FROM coaching_consultations c
                WHERE c.requester_id = gc.minor_id AND c.contract_id IS NULL AND c.payment_state <> 'unpaid'
                UNION ALL
                SELECT max(n.purchased_at) FROM guardian_notices n
                WHERE n.minor_id = gc.minor_id AND n.kind = 'purchase'
            ) x
        )
    WHERE gc.minor_id = sqlc.arg(user_id)
    RETURNING gc.minor_id
)
DELETE FROM guardian_manage_links l USING marked WHERE l.minor_id = marked.minor_id;

-- name: DeleteExpiredGuardianRecords :execrows
-- 保管期間（cutoff より前）を過ぎた、退会した本人の保護者同意の証跡を、同意・証跡・メールの記録まとめて消す
-- （変更・撤回用のリンクは guardian_consents と一緒に消える）。退会していない本人の分は対象にしない。
-- 起算点は最後の完了した取引、取引がなければ同意した日時、同意もなければ最後の記録（リンクの送信・同意しない・撤回など）の日時。
WITH expired AS (
    SELECT gc.minor_id FROM guardian_consents gc
    WHERE gc.account_deleted_at IS NOT NULL
      AND COALESCE(gc.last_transaction_at, gc.consented_at,
            GREATEST(gc.updated_at, (SELECT max(e.created_at) FROM guardian_consent_events e WHERE e.minor_id = gc.minor_id)))
          < sqlc.arg(cutoff)::timestamptz
),
events AS (DELETE FROM guardian_consent_events WHERE minor_id IN (SELECT minor_id FROM expired)),
notices AS (DELETE FROM guardian_notices WHERE minor_id IN (SELECT minor_id FROM expired))
DELETE FROM guardian_consents WHERE minor_id IN (SELECT minor_id FROM expired);
