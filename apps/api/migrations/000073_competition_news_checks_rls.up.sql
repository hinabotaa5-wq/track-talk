-- 000024 で作った competition_news_checks だけ RLS が有効になっていなかった。ほかの表と同じく、
-- 公開されている anon key で PostgREST から読み書きできないようにする（API は RLS を通らない接続で使う）。
ALTER TABLE competition_news_checks ENABLE ROW LEVEL SECURITY;
