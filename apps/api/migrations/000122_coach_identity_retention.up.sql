-- コーチの登録情報(本名・住所・電話番号)の保持期間。
-- 最後の取引が終わった時と、コーチをやめた時の遅いほうから5年たったら消す(日次の処理)。
-- 開示請求・紛争対応のために残すので、アカウントを消しても登録情報は残す(プロフィールが消えても行を消さない)。
ALTER TABLE coach_legal_identities DROP CONSTRAINT coach_legal_identities_coach_id_fkey;
ALTER TABLE coach_legal_identities
    -- コーチをやめた日時(本人がやめた・審査で却下・退会)。もう一度応募したら空に戻す。
    ADD COLUMN coaching_ended_at timestamptz,
    -- やめた時点で分かっていた最後の取引の日時。退会で取引の記録から本人が外れても起算点が分かるように残す。
    ADD COLUMN last_transaction_at timestamptz;
CREATE INDEX coach_legal_identities_ended_idx ON coach_legal_identities (coaching_ended_at) WHERE coaching_ended_at IS NOT NULL;

-- すでにやめているコーチは、やめた日時が分からないので、最後に更新した日時を使う。
UPDATE coach_legal_identities li
SET coaching_ended_at = cp.updated_at
FROM coach_profiles cp
WHERE cp.profile_id = li.coach_id AND cp.status IN ('rejected', 'suspended', 'withdrawn');
