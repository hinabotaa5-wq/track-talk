-- 商品の公式ページ。空のときは画面にリンクを出さない。
ALTER TABLE products
    ADD COLUMN product_url text NOT NULL DEFAULT ''
    CHECK (char_length(product_url) <= 500 AND (product_url = '' OR product_url LIKE 'https://%'));

-- 型落ちの商品。レビューは残したまま、一覧では新しい商品の後ろに回す。
ALTER TABLE products
    ADD COLUMN discontinued_at timestamptz;

-- 「この商品がない」リクエスト(source = 'product_request')を問い合わせと同じ受け皿に載せる。
ALTER TABLE inquiries DROP CONSTRAINT IF EXISTS inquiries_source_check;
ALTER TABLE inquiries
    ADD CONSTRAINT inquiries_source_check
    CHECK (source IN ('contact', 'feedback', 'product_request'));
