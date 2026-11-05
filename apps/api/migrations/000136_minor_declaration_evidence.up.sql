-- 未成年の購入ごとの宣言を、年齢確認・同意確認の措置を講じたことの証跡として足りる形にする。
-- これまでの記録（本人・対象の取引の種類と id・文案の版・宣言を受けた日時）に、次を足す：
--   ・その時に画面に出していた宣言文と注意書きの文面（版だけでなく文面そのもの）
--   ・対象の取引の金額とコーチ
--   ・宣言の時点で本人が登録していた生まれた年月（退会の時に書き写す birth_month とは別に持つ）
--   ・画面に宣言を出した時刻と、チェックを入れた時刻（利用者の端末の時計による申告値）
-- このマイグレーションより前の行は、足した列が空のまま残る。記録は書き換えない。
-- 記録は証跡のためのもので、返金の方針（取消し・返金申立ては全額返金で受ける）は変えない。
ALTER TABLE minor_purchase_declarations
    ADD COLUMN declaration_text text,
    ADD COLUMN notice_text text,
    ADD COLUMN amount_yen integer,
    ADD COLUMN coach_id uuid,
    ADD COLUMN declared_birth_month date,
    ADD COLUMN client_shown_at timestamptz,
    ADD COLUMN client_checked_at timestamptz;
