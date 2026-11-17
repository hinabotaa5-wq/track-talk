-- #898: 継続プランのコーチ不在日の合意延長。
-- コーチが事前に申告した対応不能期間について、選手（未成年は保護者）とコーチの明示合意があった場合だけ、
-- 契約終了日を不在日数分だけ後ろ倒しする。追加料金なし・価格と内容は不変。合意がなければ終了日は変えない。
-- 日割りの分母（期間日数）は当初の契約期間のまま固定し、合意した不在日は経過日数に数えない。

-- 合意が成立した不在期間は契約にも写す（精算の計算で契約の行だけを見れば足りるようにする）。
-- 延長は原則1回・累計7日以内なので、契約には合意済みの1件だけを持つ。履歴は下の表に全件残す。
-- absence_return_on は復帰日（不在日に含めない）。不在日は absence_from 以上 absence_return_on 未満の各日。
ALTER TABLE coaching_contracts
    ADD COLUMN absence_from date,
    ADD COLUMN absence_return_on date,
    ADD COLUMN absence_days integer NOT NULL DEFAULT 0 CHECK (absence_days BETWEEN 0 AND 7),
    ADD CONSTRAINT coaching_contracts_absence_check CHECK (
        (absence_from IS NULL AND absence_return_on IS NULL AND absence_days = 0)
        OR (absence_from IS NOT NULL AND absence_return_on IS NOT NULL AND absence_days = absence_return_on - absence_from AND absence_days >= 1)
    );

-- 申告から合意（または不成立）までの変更履歴。契約の終了日を上書きするだけでなく、元の終了日・新しい終了日・同意した人と日時を残す。
-- 取引に関する記録なので、退会後も残す（契約・プロフィールの削除と切り離す。000174 と同型）。
CREATE TABLE coaching_absence_extensions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    contract_id uuid REFERENCES coaching_contracts (id) ON DELETE SET NULL,
    coach_id uuid REFERENCES profiles (id) ON DELETE SET NULL,
    requester_id uuid REFERENCES profiles (id) ON DELETE SET NULL,
    -- declared: 申告済みで合意待ち、agreed: 合意が成立し終了日を更新した、withdrawn: コーチが取り下げた、
    -- declined: 選手が延長しないと答えた、lapsed: 復帰予定日までに合意がそろわなかった。
    status text NOT NULL DEFAULT 'declared' CHECK (status IN ('declared', 'agreed', 'withdrawn', 'declined', 'lapsed')),
    absent_from date NOT NULL,
    return_on date NOT NULL,
    absent_days integer NOT NULL CHECK (absent_days BETWEEN 1 AND 7),
    -- 理由の概要だけ。病名・診断等の詳細は取得しない。
    reason text NOT NULL CHECK (char_length(btrim(reason)) BETWEEN 1 AND 200),
    minor boolean NOT NULL DEFAULT false,
    price_yen integer NOT NULL,
    -- 提示した時点の終了日の案と、終了した場合の返金概算額（提示の写し）。
    original_ends_at timestamptz NOT NULL,
    proposed_ends_at timestamptz NOT NULL,
    refund_estimate_yen integer NOT NULL CHECK (refund_estimate_yen >= 0),
    declared_at timestamptz NOT NULL DEFAULT now(),
    -- 同意。選手（未成年は保護者も）とコーチの全員がそろったときだけ終了日を更新する。
    player_consented_by uuid REFERENCES profiles (id) ON DELETE SET NULL,
    player_consented_at timestamptz,
    guardian_consented_at timestamptz,
    coach_consented_by uuid REFERENCES profiles (id) ON DELETE SET NULL,
    coach_consented_at timestamptz,
    -- 合意が成立した時点の、実際に更新した終了日と、コーチへ再明示した取引条件。
    agreed_at timestamptz,
    agreed_original_ends_at timestamptz,
    agreed_new_ends_at timestamptz,
    coach_terms jsonb,
    closed_at timestamptz,
    CHECK (return_on - absent_from = absent_days),
    CHECK ((status = 'agreed') = (agreed_at IS NOT NULL)),
    CHECK (status <> 'agreed' OR (player_consented_at IS NOT NULL AND coach_consented_at IS NOT NULL AND agreed_new_ends_at IS NOT NULL)),
    CHECK (status <> 'agreed' OR NOT minor OR guardian_consented_at IS NOT NULL)
);
CREATE INDEX coaching_absence_extensions_contract_idx ON coaching_absence_extensions (contract_id, declared_at);
-- 申告中（合意待ち）は契約ごとに1件まで。
CREATE UNIQUE INDEX coaching_absence_extensions_open_idx ON coaching_absence_extensions (contract_id) WHERE status = 'declared';
-- 合意が成立するのは契約ごとに1回まで。
CREATE UNIQUE INDEX coaching_absence_extensions_agreed_idx ON coaching_absence_extensions (contract_id) WHERE status = 'agreed';
ALTER TABLE coaching_absence_extensions ENABLE ROW LEVEL SECURITY;

-- 契約チャットに記録する不在の申告・合意の通知は、コーチの返信とは数えない
-- （返信がない契約の自動返金の判定に使われないよう、印を付ける）。
ALTER TABLE coaching_messages ADD COLUMN system_kind text CHECK (system_kind IN ('absence_declared', 'absence_agreed', 'absence_closed'));
