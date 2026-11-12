-- 動画添削の再設計（本体 #569 フェーズA）。
-- 単発は1回500円（報酬は70%の350円）。チケットは6枚つづり2,500円で、1枚で添削1回。
-- チケットは決済完了から6か月で失効する。失効した枚数は行を消さずに expired に移し、台帳に 'expire' を残す。
-- 既存の購入・ロット・予約の金額と枚数は書き換えない（旧商品の購入済みロットは expires_at を空のままにし、失効させない）。

ALTER TABLE review_requests ALTER COLUMN base_price_yen SET DEFAULT 500;

-- 旧商品（1枚・5枚）は売らない。行は過去の購入が参照しているので消さない。
UPDATE coaching_ticket_products SET active = false WHERE id IN ('single', 'five');
INSERT INTO coaching_ticket_products (id, title, quantity, amount_yen, active)
VALUES ('six', 'チケット6枚', 6, 2500, true)
ON CONFLICT (id) DO UPDATE SET title = EXCLUDED.title, quantity = EXCLUDED.quantity, amount_yen = EXCLUDED.amount_yen, active = true;

-- 失効日時（決済完了日時の6か月後）と失効した枚数。
ALTER TABLE coaching_ticket_lots
    ADD COLUMN expires_at timestamptz,
    ADD COLUMN expired integer NOT NULL DEFAULT 0 CHECK (expired >= 0);
DO $$
DECLARE c text;
BEGIN
    FOR c IN SELECT conname FROM pg_constraint
        WHERE conrelid = 'coaching_ticket_lots'::regclass AND contype = 'c'
          AND pg_get_constraintdef(oid) LIKE '%quantity = %available%'
    LOOP
        EXECUTE format('ALTER TABLE coaching_ticket_lots DROP CONSTRAINT %I', c);
    END LOOP;
END $$;
ALTER TABLE coaching_ticket_lots ADD CONSTRAINT coaching_ticket_lots_balance_check
    CHECK (quantity = available + reserved + consumed + refunded + expired);
CREATE INDEX coaching_ticket_lots_expiry_idx ON coaching_ticket_lots (expires_at) WHERE expires_at IS NOT NULL AND available > 0;

ALTER TABLE coaching_ticket_ledger ADD COLUMN expired_delta integer NOT NULL DEFAULT 0;
DO $$
DECLARE c text;
BEGIN
    FOR c IN SELECT conname FROM pg_constraint
        WHERE conrelid = 'coaching_ticket_ledger'::regclass AND contype = 'c'
          AND pg_get_constraintdef(oid) LIKE '%kind%'
    LOOP
        EXECUTE format('ALTER TABLE coaching_ticket_ledger DROP CONSTRAINT %I', c);
    END LOOP;
END $$;
ALTER TABLE coaching_ticket_ledger ADD CONSTRAINT coaching_ticket_ledger_kind_check
    CHECK (kind IN ('purchase', 'reserve', 'consume', 'release', 'refund', 'refund_failed', 'expire'));

-- 18歳未満がチケットを買うときも、単発と同じ購入のたびの宣言を残す。
DO $$
DECLARE c text;
BEGIN
    FOR c IN SELECT conname FROM pg_constraint
        WHERE conrelid = 'minor_purchase_declarations'::regclass AND contype = 'c'
          AND pg_get_constraintdef(oid) LIKE '%ref_type%'
    LOOP
        EXECUTE format('ALTER TABLE minor_purchase_declarations DROP CONSTRAINT %I', c);
    END LOOP;
END $$;
ALTER TABLE minor_purchase_declarations ADD CONSTRAINT minor_purchase_declarations_ref_type_check
    CHECK (ref_type IN ('review', 'consultation', 'ticket_purchase'));
