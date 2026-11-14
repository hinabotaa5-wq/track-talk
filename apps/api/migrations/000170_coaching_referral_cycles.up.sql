-- 紹介リワードの周単位化（本体 #686）。
-- 000167 では紹介IDの行が1人1行で、IDと「周」を同一視していた。ここでは
-- 紹介IDの1行を1つの周として扱い、達成したIDを終了（closed_at）させ、次の起点購入で
-- 新しいIDを発行できるようにする。同時に有効なIDは1人1つだけ（部分一意索引）。
-- 終了したIDの行は残すため、新しいIDの重複照合は全履歴が対象になる（code の UNIQUE）。

ALTER TABLE coaching_referral_codes DROP CONSTRAINT coaching_referral_codes_pkey;
ALTER TABLE coaching_referral_codes
    ADD COLUMN closed_at timestamptz,
    -- 起点購入の仮押さえが解放されて無効になった周。closed_at も同時に入る。
    ADD COLUMN voided_at timestamptz,
    -- 起点購入。000170 より前に発行されたID（移行対象の周）は NULL で、起点を問わない。
    ADD COLUMN origin_ticket_purchase_id uuid UNIQUE REFERENCES coaching_ticket_purchases(id),
    ADD COLUMN origin_review_payment_id uuid UNIQUE REFERENCES review_payments(id) ON DELETE SET NULL,
    ADD CONSTRAINT coaching_referral_code_origin
        CHECK (num_nonnulls(origin_ticket_purchase_id, origin_review_payment_id) <= 1);
CREATE UNIQUE INDEX coaching_referral_active_code ON coaching_referral_codes(owner_id) WHERE closed_at IS NULL;
CREATE INDEX coaching_referral_codes_owner_idx ON coaching_referral_codes(owner_id, issued_at);

-- カウントと特典を周（紹介ID）に所属させる。既存行は1人1行時代のIDへ写す。
ALTER TABLE coaching_referral_counts
    ADD COLUMN referral_code text REFERENCES coaching_referral_codes(code) ON DELETE SET NULL;
UPDATE coaching_referral_counts a SET referral_code = c.code
FROM coaching_referral_codes c WHERE c.owner_id = a.referrer_id;
CREATE INDEX coaching_referral_counts_code_idx ON coaching_referral_counts(referral_code);

-- 重複排除は紹介者単位で生涯（周をまたぐ）。ただし取り消されたカウントは含めない。
ALTER TABLE coaching_referral_counts DROP CONSTRAINT coaching_referral_counts_referrer_id_buyer_id_key;
CREATE UNIQUE INDEX coaching_referral_counts_valid_pair
    ON coaching_referral_counts(referrer_id, buyer_id) WHERE canceled_at IS NULL;

ALTER TABLE coaching_referral_rewards
    ADD COLUMN referral_code text REFERENCES coaching_referral_codes(code) ON DELETE SET NULL;
UPDATE coaching_referral_rewards r SET referral_code = c.code
FROM coaching_referral_codes c WHERE c.owner_id = r.referrer_id;
DROP INDEX coaching_referral_active_reward;
CREATE UNIQUE INDEX coaching_referral_active_reward
    ON coaching_referral_rewards(referral_code) WHERE canceled_at IS NULL;

-- 付与済み（取り消されていない）の周は終了させる。取り消された特典の周は、
-- 000167 と同じく友達の購入で再び達成できるよう開いたままにする。
UPDATE coaching_referral_codes c SET closed_at = r.granted_at
FROM coaching_referral_rewards r WHERE r.referral_code = c.code AND r.canceled_at IS NULL;

-- owner に有効なIDがなければ、新しいIDを1つ発行する。終了したIDを含む全履歴と
-- 重ならないコードになるまで引き直す。起点購入は記録する（同じ購入で2回は発行しない）。
CREATE FUNCTION coaching_issue_referral_code(owner uuid, source text, ticket_id uuid, payment_id uuid)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    alphabet constant text := '23456789ABCDEFGHJKLMNPQRSTUVWXYZ';
    entropy bytea;
    candidate text;
BEGIN
    IF EXISTS (SELECT FROM coaching_referral_codes
               WHERE origin_ticket_purchase_id = ticket_id OR origin_review_payment_id = payment_id) THEN
        RETURN;
    END IF;
    WHILE NOT EXISTS (SELECT FROM coaching_referral_codes WHERE owner_id = owner AND closed_at IS NULL) LOOP
        entropy := uuid_send(gen_random_uuid());
        SELECT string_agg(substr(alphabet, 1 + get_byte(entropy, n) % length(alphabet), 1), '' ORDER BY n)
            INTO candidate FROM generate_series(0, 5) n;
        INSERT INTO coaching_referral_codes(owner_id, code, issued_by, origin_ticket_purchase_id, origin_review_payment_id)
        VALUES (owner, candidate, source, ticket_id, payment_id) ON CONFLICT DO NOTHING;
    END LOOP;
END
$$;

-- 000167 の API（移行先行のロールアウト中に動く旧版）が呼ぶ形を残す。起点は記録しない。
CREATE OR REPLACE FUNCTION coaching_issue_referral(owner uuid, source text) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    PERFORM coaching_issue_referral_code(owner, source, NULL, NULL);
END
$$;

-- 周の達成を判定し、達成なら特典を付与してIDを終了する。呼び出し側が紹介IDの行を
-- FOR NO KEY UPDATE で押さえていること（同じ周の2人目が同時に来ても直列になる）。
CREATE FUNCTION coaching_grant_referral(cycle text) RETURNS uuid LANGUAGE plpgsql AS $$
DECLARE
    c coaching_referral_codes;
    qualified uuid[];
    reward uuid;
BEGIN
    SELECT * INTO c FROM coaching_referral_codes WHERE code = cycle;
    IF c.code IS NULL OR c.closed_at IS NOT NULL THEN
        RETURN NULL;
    END IF;
    SELECT array_agg(id ORDER BY counted_at, id) INTO qualified
        FROM coaching_referral_counts WHERE referral_code = cycle AND canceled_at IS NULL;
    IF COALESCE(cardinality(qualified), 0) < 2 THEN
        RETURN NULL;
    END IF;
    -- 起点購入の支払いが確定していなければ付与しない（パックは支払い完了、単発は capture 済み）。
    IF c.origin_ticket_purchase_id IS NOT NULL AND NOT EXISTS (
        SELECT FROM coaching_ticket_purchases WHERE id = c.origin_ticket_purchase_id AND paid_at IS NOT NULL
    ) OR c.origin_review_payment_id IS NOT NULL AND NOT EXISTS (
        SELECT FROM review_payments WHERE id = c.origin_review_payment_id AND captured_at IS NOT NULL
    ) THEN
        RETURN NULL;
    END IF;
    reward := gen_random_uuid();
    INSERT INTO coaching_ticket_lots(id, owner_id, quantity, available, expires_at, kind)
    VALUES (reward, c.owner_id, 1, 1, now() + interval '6 months', 'referral_reward');
    INSERT INTO coaching_ticket_ledger(id, lot_id, operation_key, kind, available_delta)
    VALUES (gen_random_uuid(), reward, 'referral-grant-' || reward, 'referral_grant', 1);
    INSERT INTO coaching_referral_rewards(lot_id, referrer_id, first_count_id, second_count_id, referral_code)
    VALUES (reward, c.owner_id, qualified[1], qualified[2], cycle);
    UPDATE coaching_referral_codes SET closed_at = now() WHERE code = cycle;
    RETURN c.owner_id;
END
$$;

-- 入力された紹介IDで購入を数える。数えたら周の達成を判定し、付与した紹介者を返す。
-- 存在しない・自分の・終了したID、その紹介者に有効カウント済みの購入者は数えない。
CREATE OR REPLACE FUNCTION coaching_count_referral(buyer uuid, entered_code text, ticket_id uuid, payment_id uuid)
RETURNS uuid LANGUAGE plpgsql AS $$
DECLARE
    referrer uuid;
    closed timestamptz;
BEGIN
    SELECT owner_id, closed_at INTO referrer, closed FROM coaching_referral_codes
        WHERE code = entered_code FOR NO KEY UPDATE;
    IF referrer IS NULL OR referrer = buyer OR closed IS NOT NULL THEN
        RETURN NULL;
    END IF;
    INSERT INTO coaching_referral_counts(referrer_id, buyer_id, ticket_purchase_id, review_payment_id, referral_code)
    VALUES (referrer, buyer, ticket_id, payment_id, entered_code) ON CONFLICT DO NOTHING;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;
    RETURN coaching_grant_referral(entered_code);
END
$$;

-- 確定した対象購入1件を記録する。入力された紹介IDで数え、その購入に有効カウントが
-- 紐付かなかったときだけ、購入者の起点購入として紹介IDを発行する（有効なIDがあれば何もしない）。
-- 紹介者のID行のロックと購入者のID発行は別の行なので、相互紹介でも逆順ロックにならない。
CREATE FUNCTION coaching_record_referral(buyer uuid, entered_code text, ticket_id uuid, payment_id uuid)
RETURNS uuid LANGUAGE plpgsql AS $$
DECLARE
    recipient uuid;
    origin_code text;
BEGIN
    IF entered_code IS NOT NULL THEN
        recipient := coaching_count_referral(buyer, entered_code, ticket_id, payment_id);
    END IF;
    IF NOT EXISTS (
        SELECT FROM coaching_referral_counts
        WHERE (ticket_purchase_id = ticket_id OR review_payment_id = payment_id) AND canceled_at IS NULL
    ) THEN
        PERFORM coaching_issue_referral_code(buyer, 'purchase', ticket_id, payment_id);
    END IF;
    -- この購入が既存の周の起点で、確定を待って付与を保留していたなら、ここで付与する。
    -- （現行の API はIDを支払い確定時に発行するため、通常は保留が起きない。DB 側の担保。）
    IF recipient IS NULL THEN
        SELECT code INTO origin_code FROM coaching_referral_codes
            WHERE (origin_ticket_purchase_id = ticket_id OR origin_review_payment_id = payment_id)
              AND closed_at IS NULL
            FOR NO KEY UPDATE;
        IF origin_code IS NOT NULL THEN
            recipient := coaching_grant_referral(origin_code);
        END IF;
    END IF;
    RETURN recipient;
END
$$;

-- カウント1件を取り消し、そのカウントで付与された未使用の特典を取り消す。
-- 返金（トリガー）とチャージバック（webhook）の両方から呼ぶ。
CREATE FUNCTION coaching_cancel_referral_count(count_id uuid) RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    contribution coaching_referral_counts;
    reward coaching_referral_rewards;
    lot coaching_ticket_lots;
BEGIN
    SELECT * INTO contribution FROM coaching_referral_counts WHERE id = count_id;
    IF contribution.id IS NULL THEN
        RETURN;
    END IF;
    PERFORM 1 FROM coaching_referral_codes WHERE code = contribution.referral_code FOR NO KEY UPDATE;
    UPDATE coaching_referral_counts SET canceled_at = now() WHERE id = contribution.id AND canceled_at IS NULL;
    IF NOT FOUND THEN
        RETURN;
    END IF;

    SELECT * INTO reward FROM coaching_referral_rewards
        WHERE canceled_at IS NULL AND contribution.id IN (first_count_id, second_count_id);
    IF NOT FOUND THEN
        RETURN;
    END IF;
    SELECT * INTO lot FROM coaching_ticket_lots WHERE id = reward.lot_id FOR UPDATE;
    UPDATE coaching_referral_rewards SET qualifying_refund_at = now(),
        canceled_at = CASE WHEN lot.consumed = 0 AND lot.reserved = 0 THEN now() END
    WHERE lot_id = lot.id;
    -- Honor an existing reservation. On release, the API checks this
    -- recorded refund and discards the ticket instead of restoring it.
    IF lot.available > 0 THEN
        UPDATE coaching_ticket_lots SET available = 0, expired = expired + lot.available
        WHERE id = lot.id;
        INSERT INTO coaching_ticket_ledger(id, lot_id, operation_key, kind, available_delta, expired_delta)
        VALUES (gen_random_uuid(), lot.id, 'referral-cancel-' || lot.id, 'expire', -lot.available, lot.available);
    END IF;
END
$$;

CREATE OR REPLACE FUNCTION coaching_cancel_referral() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    count_id uuid;
    origin_code text;
BEGIN
    IF TG_TABLE_NAME = 'review_payments' THEN
        -- 起点購入の仮押さえが解放された（本人の支払いが発生しなかった）なら、その周を無効にする。
        -- 周で立っていたカウントは取り消し扱いにし、友達側の重複排除を消費しない。
        IF NEW.status = 'canceled' AND OLD.status = 'authorized' THEN
            SELECT code INTO origin_code FROM coaching_referral_codes
                WHERE origin_review_payment_id = NEW.id AND closed_at IS NULL FOR NO KEY UPDATE;
            IF origin_code IS NOT NULL THEN
                UPDATE coaching_referral_codes SET closed_at = now(), voided_at = now() WHERE code = origin_code;
                UPDATE coaching_referral_counts SET canceled_at = now()
                    WHERE referral_code = origin_code AND canceled_at IS NULL;
            END IF;
            RETURN NEW;
        END IF;
        IF NEW.status <> 'refunded' OR OLD.status = 'refunded' THEN
            RETURN NEW;
        END IF;
        SELECT id INTO count_id FROM coaching_referral_counts WHERE review_payment_id = NEW.id;
    ELSE
        IF NEW.refunded_yen <= OLD.refunded_yen THEN
            RETURN NEW;
        END IF;
        SELECT id INTO count_id FROM coaching_referral_counts WHERE ticket_purchase_id = NEW.id;
    END IF;
    IF count_id IS NOT NULL THEN
        PERFORM coaching_cancel_referral_count(count_id);
    END IF;
    RETURN NEW;
END
$$;

-- チャージバックで失効した支払いのカウントを、返金と同じく取り消す。
CREATE FUNCTION coaching_cancel_referral_for_payment_intent(intent text) RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    count_id uuid;
BEGIN
    FOR count_id IN
        SELECT a.id FROM coaching_referral_counts a
        JOIN coaching_ticket_purchases p ON p.id = a.ticket_purchase_id
        WHERE p.payment_intent_id = intent AND a.canceled_at IS NULL
        UNION
        SELECT a.id FROM coaching_referral_counts a
        JOIN review_payments p ON p.id = a.review_payment_id
        WHERE p.payment_intent_id = intent AND a.canceled_at IS NULL
    LOOP
        PERFORM coaching_cancel_referral_count(count_id);
    END LOOP;
END
$$;

-- 特典が取り消されたら、その周を開き直して友達の購入で再び達成できるようにする
-- （000167 の再付与と同じ）。ただし本人がすでに次の周の有効なIDを持っているときは
-- 同時に有効なIDを2つにしないため、取り消された周は終了のままにする。
CREATE FUNCTION coaching_reopen_referral_cycle() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.canceled_at IS NOT NULL OR NEW.canceled_at IS NULL OR NEW.referral_code IS NULL THEN
        RETURN NEW;
    END IF;
    BEGIN
        UPDATE coaching_referral_codes c SET closed_at = NULL
        WHERE c.code = NEW.referral_code AND c.closed_at IS NOT NULL AND NOT EXISTS (
            SELECT FROM coaching_referral_codes o WHERE o.owner_id = c.owner_id AND o.closed_at IS NULL
        );
    EXCEPTION WHEN unique_violation THEN
        NULL;
    END;
    RETURN NEW;
END
$$;
CREATE TRIGGER coaching_referral_reward_canceled AFTER UPDATE OF canceled_at ON coaching_referral_rewards
    FOR EACH ROW EXECUTE FUNCTION coaching_reopen_referral_cycle();
