-- プランで「何が返ってくるか」。利用者がコーチ同士を比べやすいよう、説明文とは別に決まった形で持つ。
-- どれも NULL（空）なら「書いていない」。前からあるプランはそのまま書いていない扱いにする。
-- video_reviews: 動画への返答の回数。0 は回数を決めない（期間中いつでも）、1〜30 はその回数まで。
-- menu_updates: none はメニューなし、once は最初に1回作る、weekly は毎週見直す。
-- reply_formats: 返し方。text は文章、video は解説の動画。
ALTER TABLE coaching_plans
    ADD COLUMN video_reviews smallint CHECK (video_reviews BETWEEN 0 AND 30),
    ADD COLUMN menu_updates text CHECK (menu_updates IN ('none', 'once', 'weekly')),
    ADD COLUMN reply_formats text[] NOT NULL DEFAULT '{}' CHECK (reply_formats <@ ARRAY['text', 'video']);

-- 申し込んだ時点の写し。あとでコーチがプランを直しても、この申し込みで約束した中身は変わらない。
ALTER TABLE coaching_contracts
    ADD COLUMN video_reviews smallint CHECK (video_reviews BETWEEN 0 AND 30),
    ADD COLUMN menu_updates text CHECK (menu_updates IN ('none', 'once', 'weekly')),
    ADD COLUMN reply_formats text[] NOT NULL DEFAULT '{}' CHECK (reply_formats <@ ARRAY['text', 'video']);
