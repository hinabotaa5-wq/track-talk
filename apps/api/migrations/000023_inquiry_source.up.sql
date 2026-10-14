ALTER TABLE inquiries
    ADD COLUMN source text NOT NULL DEFAULT 'contact'
    CHECK (source IN ('contact', 'feedback'));

-- 「なんでも書いてねボックス」は本文の先頭80文字を件名にして送る。
UPDATE inquiries
SET source = 'feedback'
WHERE category = 'other'
  AND subject = left(body, 80);

CREATE INDEX inquiries_source_created_idx ON inquiries (source, created_at DESC);
