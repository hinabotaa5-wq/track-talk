UPDATE inquiries SET category = 'other' WHERE category = 'disclosure';
ALTER TABLE inquiries DROP CONSTRAINT inquiries_category_check;
ALTER TABLE inquiries ADD CONSTRAINT inquiries_category_check
    CHECK (category IN ('bug', 'account', 'request', 'other'));
