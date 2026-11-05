-- 18歳未満が自分で払う動画添削・追加相談は、保護者同意と月の上限の代わりに、購入のたびに
-- 「保護者に口頭で許可をもらいました」の宣言を必須にする。その宣言を取引ごとに1行ずつ残す（書き換えない）。
-- 支払い画面を作り直すたびに宣言し直すので、同じ購入に複数の行が残ることがある。
-- 保管期間は保護者同意の記録と同じ「最後の取引から5年」。退会でプロフィールが消えても残すため、profiles への外部キーは付けない。
CREATE TABLE minor_purchase_declarations (
    id uuid PRIMARY KEY,
    minor_id uuid NOT NULL,
    -- 対象の購入。review は動画添削の依頼（review_requests.id）、consultation は追加相談（coaching_consultations.id）。
    ref_type text NOT NULL CHECK (ref_type IN ('review', 'consultation')),
    ref_id uuid NOT NULL,
    -- 宣言の文案の版。画面の文案を変えたら上げる。
    text_version text NOT NULL,
    declared_at timestamptz NOT NULL DEFAULT now(),
    -- 本人が退会した日時と、その時点での最後の完了した取引の日時（guardian_consents と同じ考え方）。
    account_deleted_at timestamptz,
    last_transaction_at timestamptz
);
CREATE INDEX minor_purchase_declarations_minor_idx ON minor_purchase_declarations (minor_id, declared_at);
CREATE INDEX minor_purchase_declarations_ref_idx ON minor_purchase_declarations (ref_type, ref_id);
CREATE INDEX minor_purchase_declarations_deleted_idx ON minor_purchase_declarations (account_deleted_at) WHERE account_deleted_at IS NOT NULL;
-- API からだけ読み書きする。anon・authenticated からは見えないようにする。
ALTER TABLE minor_purchase_declarations ENABLE ROW LEVEL SECURITY;

-- 18歳未満の本人が自分で払った動画添削の支払いに印を付ける。返金の件数・金額を、未成年の購入の分だけ後から集計できるようにする。
-- 退会で payer_id が空になっても印は残る。追加相談は coaching_consultations.minor を使う。
ALTER TABLE review_payments ADD COLUMN minor boolean NOT NULL DEFAULT false;
