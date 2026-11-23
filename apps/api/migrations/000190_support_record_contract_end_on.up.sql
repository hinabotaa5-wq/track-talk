-- 本体 #981: 取引条件の明示（support_contract_statements）と精算記録（仮払い・確定・帰責判定）に保存期限を付ける。
-- 削除できるのは、作成した年の8年後の3月16日（日本時間）と、契約終了日の7年後の翌日の、遅い方を過ぎてから。
-- 日次の削除ジョブが消すのは契約がすでに消えた（contract_id が NULL の）記録だけなので、契約終了日はこの控え列から取る。
-- 契約終了日は、途中終了の日割りの終わりの日（support_end_on）と ends_at の日本時間の日付の遅い方。
ALTER TABLE support_contract_statements ADD COLUMN contract_end_on date;
ALTER TABLE support_coach_settlements ADD COLUMN contract_end_on date;
ALTER TABLE support_coach_advances ADD COLUMN contract_end_on date;
ALTER TABLE support_fault_determinations ADD COLUMN contract_end_on date;

-- 契約が残っている記録には、いまの契約終了日を書き込む（終了日が決まっていない契約は NULL のまま）。
UPDATE support_contract_statements r SET contract_end_on = GREATEST(c.support_end_on, (c.ends_at AT TIME ZONE 'Asia/Tokyo')::date)
FROM coaching_contracts c WHERE c.id = r.contract_id;
UPDATE support_coach_settlements r SET contract_end_on = GREATEST(c.support_end_on, (c.ends_at AT TIME ZONE 'Asia/Tokyo')::date)
FROM coaching_contracts c WHERE c.id = r.contract_id;
UPDATE support_coach_advances r SET contract_end_on = GREATEST(c.support_end_on, (c.ends_at AT TIME ZONE 'Asia/Tokyo')::date)
FROM coaching_contracts c WHERE c.id = r.contract_id;
UPDATE support_fault_determinations r SET contract_end_on = GREATEST(c.support_end_on, (c.ends_at AT TIME ZONE 'Asia/Tokyo')::date)
FROM coaching_contracts c WHERE c.id = r.contract_id;

-- 契約の行が消える時（退会の CASCADE を含む全経路）に、外部キーで contract_id が NULL になる前の記録へ、
-- 消える時点の契約終了日を書き写す。終了日が決まらないまま消える契約は、消えた日（日本時間）を書き写す。
CREATE FUNCTION copy_support_record_contract_end_on() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    end_on date := COALESCE(GREATEST(OLD.support_end_on, (OLD.ends_at AT TIME ZONE 'Asia/Tokyo')::date),
                            (now() AT TIME ZONE 'Asia/Tokyo')::date);
BEGIN
    UPDATE support_contract_statements SET contract_end_on = end_on WHERE contract_id = OLD.id;
    UPDATE support_coach_settlements SET contract_end_on = end_on WHERE contract_id = OLD.id;
    UPDATE support_coach_advances SET contract_end_on = end_on WHERE contract_id = OLD.id;
    UPDATE support_fault_determinations SET contract_end_on = end_on WHERE contract_id = OLD.id;
    RETURN OLD;
END
$$;
CREATE TRIGGER copy_support_record_contract_end_on BEFORE DELETE ON coaching_contracts
    FOR EACH ROW EXECUTE FUNCTION copy_support_record_contract_end_on();
