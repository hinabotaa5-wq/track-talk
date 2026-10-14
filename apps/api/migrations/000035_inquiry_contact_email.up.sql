-- お問い合わせ(source = 'contact')の返信先。既存の行と「なんでも書いてねボックス」は持たない。
ALTER TABLE inquiries
    ADD COLUMN contact_email varchar(254);
