-- マイグレーション107適用済みの専用ローカルDBで実行する。
-- 検索語のない並べ替えの核となるSQLを比較し、データは最後に取り消す。
\set ON_ERROR_STOP on
BEGIN;
INSERT INTO profiles(id,nickname) VALUES('ffffffff-1111-4111-8111-111111111111','perf');
INSERT INTO posts(id,author_id,title,body,kind,created_at,like_count)
SELECT md5('perf-'||g)::uuid,'ffffffff-1111-4111-8111-111111111111','陸上','本文','text',timestamptz '2026-10-24 00:00:00+00' - g * interval '1 second',g%100 FROM generate_series(1,100000) g;
ANALYZE posts;
\echo legacy count
EXPLAIN (ANALYZE, BUFFERS) SELECT count(*) FROM posts WHERE deleted_at IS NULL AND hidden_at IS NULL AND kind NOT IN ('injury','meet');
\echo legacy deep offset
EXPLAIN (ANALYZE, BUFFERS) SELECT id,title,body FROM posts WHERE deleted_at IS NULL AND hidden_at IS NULL AND kind NOT IN ('injury','meet') ORDER BY created_at DESC LIMIT 13 OFFSET 90000;
\echo cursor deep new
SET LOCAL plan_cache_mode=force_generic_plan;
PREPARE feed_new(timestamptz,uuid) AS SELECT id,title,body FROM posts WHERE deleted_at IS NULL AND hidden_at IS NULL AND kind NOT IN ('injury','meet') AND (created_at,id)<($1,$2) AND created_at <= timestamptz '2026-10-24 00:00:00+00' ORDER BY created_at DESC,id DESC LIMIT 13;
EXPLAIN (ANALYZE, BUFFERS) EXECUTE feed_new(timestamptz '2026-10-24 00:00:00+00' - interval '90000 seconds','ffffffff-ffff-ffff-ffff-ffffffffffff');
\echo cursor deep popular
PREPARE feed_popular(int,timestamptz,uuid) AS SELECT id,title,body FROM posts WHERE deleted_at IS NULL AND hidden_at IS NULL AND kind NOT IN ('injury','meet') AND (like_count,created_at,id)<($1,$2,$3) AND created_at <= timestamptz '2026-10-24 00:00:00+00' AND created_at >= timestamptz '2026-10-24 00:00:00+00' - interval '14 days' ORDER BY like_count DESC,created_at DESC,id DESC LIMIT 13;
EXPLAIN (ANALYZE, BUFFERS) EXECUTE feed_popular(10,timestamptz '2026-10-24 00:00:00+00' - interval '90000 seconds','ffffffff-ffff-ffff-ffff-ffffffffffff');
ROLLBACK;
