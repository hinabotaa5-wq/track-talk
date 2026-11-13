-- 000168 の巻き戻し。通常の適用の巻き戻し用であり、データの救済ではない
-- （000162・000163・000167 の down と同じ扱い）。

DROP INDEX coaching_ticket_ledger_purchase_ref_idx;

ALTER TABLE coaching_ticket_ledger
    DROP COLUMN purchase_ref;
