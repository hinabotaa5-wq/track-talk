-- 感覚メモを書いた日を、メモごとに持つ。日付を選んで書けるようにし、1日1件にするため、
-- 保存した時刻（created_at）とは別にする。これまでのメモは、保存した日（日本時間）を書いた日にする。
ALTER TABLE sense_notes ADD COLUMN day date;

UPDATE sense_notes SET day = (created_at AT TIME ZONE 'Asia/Tokyo')::date;

ALTER TABLE sense_notes
    ALTER COLUMN day SET DEFAULT (timezone('Asia/Tokyo', now()))::date,
    ALTER COLUMN day SET NOT NULL;

CREATE INDEX sense_notes_author_day_idx ON sense_notes (author_id, day DESC, created_at DESC, id DESC);
