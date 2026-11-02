-- 練習相手の募集に都道府県を持たせ、地域と種目で一覧できるようにする。
-- 以前からある募集は空(地域未設定)のまま残す。新しい募集と編集では API が必須にする。
ALTER TABLE practice_details
    ADD COLUMN prefecture text,
    ADD CONSTRAINT practice_details_prefecture_check CHECK (
        prefecture IS NULL OR prefecture ~ '^(0[1-9]|[1-3][0-9]|4[0-7])$'
    );

-- 一覧は「これからの、解散していない募集」を日時の早い順に読む。
CREATE INDEX practice_details_upcoming_idx
    ON practice_details (starts_at, post_id)
    WHERE dissolved_at IS NULL;
