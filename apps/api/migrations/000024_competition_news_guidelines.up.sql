-- 県陸協の新着から補った要項のURL。年間表由来のリンク(organizer_url / link_label)とは別に持ち、年間日程の再取り込みで消えないようにする。
ALTER TABLE competitions
    ADD COLUMN guidelines_url text NOT NULL DEFAULT '';

-- 新着を最後に確認した日時。県を古い順に回すために使う。
CREATE TABLE competition_news_checks (
    prefecture text PRIMARY KEY,
    checked_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT competition_news_checks_prefecture_chk CHECK (prefecture ~ '^(0[1-9]|[1-3][0-9]|4[0-7])$')
);
