-- 特定商取引法の表記で省略した住所・電話番号の開示請求を、お問い合わせの種類として受け付ける。
ALTER TABLE inquiries DROP CONSTRAINT inquiries_category_check;
ALTER TABLE inquiries ADD CONSTRAINT inquiries_category_check
    CHECK (category IN ('bug', 'account', 'request', 'other', 'disclosure'));
