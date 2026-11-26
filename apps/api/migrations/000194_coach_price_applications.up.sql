-- コーチの価格申請制（本体 #1052、運営者決定 2026-11-25／2026-11-26）。
-- コーチがプラン（スタンダード／プレミアム）ごとに価格を申請し、運営が承認する。承認しても即時には切り替えず、
-- 承認の時点でそのコーチが対応中だった依頼（承認日時以前に作られ、まだ終わっていない依頼）がすべて終わってから有効にする。
-- 金額は円のみ（海外コーチも円で申請する。公開ページの $ 表示は変えない）。下限は500円、上限なし。

-- いま有効な価格。行がないプランは未設定で、スタンダードは API の reviewPriceYen（1,000円）を使う。
-- 既存コーチは行を作らない（スタンダードの有効価格は従来どおり1,000円として扱う）。
CREATE TABLE coach_prices (
    coach_id uuid NOT NULL REFERENCES coach_profiles (profile_id) ON DELETE CASCADE,
    plan text NOT NULL CHECK (plan IN ('standard', 'premium')),
    price_yen integer NOT NULL CHECK (price_yen >= 500),
    -- この価格を有効にした申請。
    application_id uuid,
    effective_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (coach_id, plan)
);

-- 申請の履歴。pending: 承認待ち（コーチが上書き・取り下げできる）。approved: 承認済みで切り替え待ち。
-- applied: 有効になった。withdrawn: コーチが取り下げた。superseded: 切り替え前に、より新しい申請が承認された。
-- 却下はしない方針（承認は確認・記録の場）なので rejected は置かない。
CREATE TABLE coach_price_applications (
    id uuid PRIMARY KEY,
    coach_id uuid NOT NULL REFERENCES coach_profiles (profile_id) ON DELETE CASCADE,
    plan text NOT NULL CHECK (plan IN ('standard', 'premium')),
    price_yen integer NOT NULL CHECK (price_yen >= 500),
    -- price: コーチ画面の価格申請。coach_apply: コーチ応募フォームの希望価格。
    source text NOT NULL CHECK (source IN ('price', 'coach_apply')),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'applied', 'withdrawn', 'superseded')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    approved_at timestamptz,
    approved_by uuid REFERENCES profiles (id) ON DELETE SET NULL,
    applied_at timestamptz,
    withdrawn_at timestamptz,
    CHECK ((status IN ('pending', 'withdrawn')) = (approved_at IS NULL)),
    CHECK ((status = 'applied') = (applied_at IS NOT NULL)),
    CHECK ((status = 'withdrawn') = (withdrawn_at IS NOT NULL))
);

-- 承認待ちはコーチ×プランごとに1件だけ（上書きはこの1件の書き換え。補足 S4）。
CREATE UNIQUE INDEX coach_price_applications_pending_idx ON coach_price_applications (coach_id, plan) WHERE status = 'pending';
-- 切り替え待ちもコーチ×プランごとに1件だけ（新しい承認が来たら古い方は superseded にする）。
CREATE UNIQUE INDEX coach_price_applications_approved_idx ON coach_price_applications (coach_id, plan) WHERE status = 'approved';
CREATE INDEX coach_price_applications_coach_idx ON coach_price_applications (coach_id, created_at DESC);

-- 切り替え判定（承認日時以前に作られた、このコーチの未終了の依頼）を引く。
CREATE INDEX review_requests_requested_coach_open_idx ON review_requests (requested_coach_id, created_at)
    WHERE status IN ('awaiting_payment', 'queued', 'offered', 'in_review', 'needs_info');

ALTER TABLE coach_prices ENABLE ROW LEVEL SECURITY;
ALTER TABLE coach_price_applications ENABLE ROW LEVEL SECURITY;
