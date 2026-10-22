-- 本番に入っているケガのデモ用プロフィール(81人)の名前と種目を、injury_demo.sql の今のものにそろえる。
-- 名前: 漢字・ひらがな・カタカナ・アルファベットと、それを組み合わせたもの。
-- 種目: グループの中の奇数番目の人を 100m にして、どのグループでも「種目でいちばん多い」が 100m になるようにする。
-- ケガの答えや段階など、ほかは変えない。何度流しても同じ結果になる。
-- ニックネームは全員で重ならない決まりなので、本物の利用者が同じ名前を使っていたら、その名前の後ろに番号を付ける。
-- 実行: psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f apps/api/seeds/injury_demo_update.sql
BEGIN;

-- デモどうしで名前を入れ替えるときに一時的に重ならないよう、いったん仮の名前にする。
UPDATE profiles
SET nickname = '#tmp' || right(id::text, 3)
WHERE id::text LIKE '00000000-0000-4000-8000-0000000000%';

UPDATE profiles p
SET nickname = CASE
        WHEN EXISTS (SELECT 1 FROM profiles q WHERE q.nickname = v.nickname AND q.id::text NOT LIKE '00000000-0000-4000-8000-0000000000%')
            THEN v.nickname || v.n
        ELSE v.nickname
    END
FROM (
    SELECT n, (ARRAY['そら', 'Haru', '陽', 'ミナト', 'kei', 'ゆい', 'リク', '蒼', 'Mio', 'はるき', 'ソウタ', '凛', 'yuto', 'あおい', 'ケンタ', '楓', 'Sora', 'ひなこ', 'カイ', '湊', 'リク07', 'めい', 'ユウキ', '大地', 'Nao', 'こはる', 'タクミ', '結衣', 'ren', 'さくら', '陽ちゃん', '翼', 'Aoi', 'りん', 'シュン', '智也', 'hina', 'ゆうと', 'ミユ', '健太', 'Taiga', 'なつき', 'コウ', '美咲', 'sho', 'いぶき', 'アカリ', '颯太', 'Yuna', 'ちひろ', 'レン', '悠斗', 'kaito', 'まなと', 'サキ', '拓海', 'Rio', 'ほのか', 'ダイチ', '優花', 'めいRun', 'つばさ', 'ナナ', '陸斗', 'Kenji', 'あやね', 'ショウ', '千夏', 'yui', 'しゅんた', 'アオ', '航', 'Kaiくん', 'ことね', 'ヒロ', '瑞希', 'haruto', 'みずき', 'ジン', '紗良', '走るタロウ'])[n] AS nickname
    FROM generate_series(1, 81) AS n
) v
WHERE p.id = ('00000000-0000-4000-8000-' || lpad(v.n::text, 12, '0'))::uuid;

-- injury_demo.sql と同じ決め方。i はグループの中の番号(デモの番号の小さい順)、n はデモの番号。
UPDATE injury_group_members m
SET event = CASE WHEN x.i % 2 = 1 THEN '100m'
        ELSE (ARRAY['200m', '400m', '800m', '1500m', '5000m', '110mh', 'high-jump', 'long-jump', 'shot-put', 'javelin', 'race-walk'])[1 + (x.n * 7) % 11]
    END
FROM (
    SELECT user_id, body_part,
        row_number() OVER (PARTITION BY body_part ORDER BY user_id) AS i,
        right(user_id::text, 12)::int AS n
    FROM injury_group_members
    WHERE user_id::text LIKE '00000000-0000-4000-8000-0000000000%'
) x
WHERE m.user_id = x.user_id AND m.body_part = x.body_part;

COMMIT;
