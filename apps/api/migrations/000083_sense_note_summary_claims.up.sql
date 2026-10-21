-- 感覚メモのまとめを作っている最中の印。AI の返事を待つあいだに同じ人のまとめをもう1つ作らせない
-- (同時に頼まれると、どれも「今週はまだ作っていない」と読んで AI を何度も呼んでしまう)。
-- 作り終えたら消す。途中で落ちて残っても、2分たてば次の人が取り直せる。
CREATE TABLE sense_note_summary_claims (
    author_id uuid PRIMARY KEY REFERENCES profiles (id) ON DELETE CASCADE,
    claimed_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE sense_note_summary_claims ENABLE ROW LEVEL SECURITY;
