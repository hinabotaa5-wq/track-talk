-- フォーム添削の追加相談も都度払い(カード・PayPay)で買えるようにする。チケットの新規販売を止めたため。
-- 購入済みチケットで払った相談('ticket')の記録はそのまま残す。テーブル・カラムは消さない。
DO $$
DECLARE c text;
BEGIN
    SELECT conname INTO c FROM pg_constraint
    WHERE conrelid = 'coaching_consultations'::regclass AND contype = 'c'
      AND pg_get_constraintdef(oid) LIKE '%review_request_id IS NOT NULL%funding%';
    IF c IS NOT NULL THEN
        EXECUTE format('ALTER TABLE coaching_consultations DROP CONSTRAINT %I', c);
    END IF;
END $$;
ALTER TABLE coaching_consultations ADD CONSTRAINT coaching_consultations_funding_parent_check
    CHECK ((review_request_id IS NOT NULL AND funding IN ('ticket','cash')) OR (contract_id IS NOT NULL AND funding='cash'));
