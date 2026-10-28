-- 保護者同意の証跡の保管期間。運営者の決定（2026-10-29）により、同意の記録と取引に関する記録は、
-- 利用者が退会しても「最後の取引から5年」残し、過ぎたら日次の処理で消す。
-- 取引が一度もないときは同意した日時から、同意がないときは最後の記録（リンクの送信など）の日時から数える。
-- 退会でプロフィールが消えても行が消えないよう、profiles への外部キーを外す（コーチの登録情報と同じやり方）。
ALTER TABLE guardian_consents DROP CONSTRAINT guardian_consents_minor_id_fkey;
ALTER TABLE guardian_consent_events DROP CONSTRAINT guardian_consent_events_minor_id_fkey;
ALTER TABLE guardian_notices DROP CONSTRAINT guardian_notices_minor_id_fkey;

ALTER TABLE guardian_consents
    -- 本人が退会した日時。退会したあとは、同意用のリンクも変更・撤回用のリンクも使えない。
    ADD COLUMN account_deleted_at timestamptz,
    -- 退会した時点で分かっていた、本人の最後の完了した取引の日時。
    -- 退会すると支払いの記録から本人が外れる（payer_id・requester_id が空になる）ので、起算点をここに残す。
    ADD COLUMN last_transaction_at timestamptz;
CREATE INDEX guardian_consents_account_deleted_idx ON guardian_consents (account_deleted_at) WHERE account_deleted_at IS NOT NULL;

-- 購入の通知に、Stripe の支払いの ID を残す。退会で依頼が消えても、どの支払いだったかを Stripe の記録と突き合わせられる。
ALTER TABLE guardian_notices ADD COLUMN payment_intent_id text;
