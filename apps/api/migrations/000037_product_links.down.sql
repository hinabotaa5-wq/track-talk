DELETE FROM inquiries WHERE source = 'product_request';
ALTER TABLE inquiries DROP CONSTRAINT IF EXISTS inquiries_source_check;
ALTER TABLE inquiries
    ADD CONSTRAINT inquiries_source_check
    CHECK (source IN ('contact', 'feedback'));
ALTER TABLE products DROP COLUMN discontinued_at;
ALTER TABLE products DROP COLUMN product_url;
