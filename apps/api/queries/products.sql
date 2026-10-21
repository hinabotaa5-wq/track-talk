-- name: ListProducts :many
-- 絞り込み(種目・レベル・足のサイズ・足幅・使い方)を付けたときは、合うレビューだけで件数と平均★を数え直し、
-- 合うレビューが1件もない商品は出さない。
SELECT
    p.id,
    p.kind,
    p.brand,
    p.name,
    p.image_url,
    p.product_url,
    (p.discontinued_at IS NOT NULL)::boolean AS discontinued,
    COALESCE(s.review_count, 0)::bigint AS review_count,
    COALESCE(s.avg_rating, 0)::float8 AS avg_rating
FROM products p
LEFT JOIN (
    SELECT r.product_id, count(*) AS review_count, avg(r.rating) AS avg_rating
    FROM product_reviews r
    WHERE r.deleted_at IS NULL AND r.hidden_at IS NULL
      AND (sqlc.narg(event)::text IS NULL OR r.event = sqlc.narg(event)::text)
      AND (sqlc.narg(level)::text IS NULL OR r.level = sqlc.narg(level)::text)
      AND (sqlc.narg(foot_size_min)::smallint IS NULL OR r.foot_size >= sqlc.narg(foot_size_min)::smallint)
      AND (sqlc.narg(foot_size_max)::smallint IS NULL OR r.foot_size <= sqlc.narg(foot_size_max)::smallint)
      AND (sqlc.narg(foot_width)::text IS NULL OR r.foot_width = sqlc.narg(foot_width)::text)
      AND (sqlc.narg(usage)::text IS NULL OR r.usage = sqlc.narg(usage)::text)
    GROUP BY r.product_id
) s ON s.product_id = p.id
WHERE p.hidden_at IS NULL
  AND p.kind = sqlc.arg(kind)
  AND (sqlc.narg(q)::text IS NULL OR p.name ILIKE '%' || sqlc.narg(q)::text || '%' ESCAPE '\' OR p.brand ILIKE '%' || sqlc.narg(q)::text || '%' ESCAPE '\')
  AND (NOT sqlc.arg(filtered)::boolean OR s.review_count > 0)
ORDER BY (p.discontinued_at IS NOT NULL) ASC, COALESCE(s.review_count, 0) DESC, COALESCE(s.avg_rating, 0) DESC, p.created_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountProducts :one
SELECT count(*)
FROM products p
WHERE p.hidden_at IS NULL
  AND (
      NOT sqlc.arg(filtered)::boolean
      OR EXISTS (
          SELECT 1 FROM product_reviews r
          WHERE r.product_id = p.id AND r.deleted_at IS NULL AND r.hidden_at IS NULL
          AND (sqlc.narg(event)::text IS NULL OR r.event = sqlc.narg(event)::text)
          AND (sqlc.narg(level)::text IS NULL OR r.level = sqlc.narg(level)::text)
          AND (sqlc.narg(foot_size_min)::smallint IS NULL OR r.foot_size >= sqlc.narg(foot_size_min)::smallint)
          AND (sqlc.narg(foot_size_max)::smallint IS NULL OR r.foot_size <= sqlc.narg(foot_size_max)::smallint)
          AND (sqlc.narg(foot_width)::text IS NULL OR r.foot_width = sqlc.narg(foot_width)::text)
          AND (sqlc.narg(usage)::text IS NULL OR r.usage = sqlc.narg(usage)::text)
      )
  )
  AND p.kind = sqlc.arg(kind)
  AND (sqlc.narg(q)::text IS NULL OR p.name ILIKE '%' || sqlc.narg(q)::text || '%' ESCAPE '\' OR p.brand ILIKE '%' || sqlc.narg(q)::text || '%' ESCAPE '\');

-- name: GetProduct :one
SELECT
    p.id,
    p.kind,
    p.brand,
    p.name,
    p.image_url,
    p.product_url,
    (p.discontinued_at IS NOT NULL)::boolean AS discontinued,
    COALESCE(s.review_count, 0)::bigint AS review_count,
    COALESCE(s.avg_rating, 0)::float8 AS avg_rating
FROM products p
LEFT JOIN (
    SELECT product_id, count(*) AS review_count, avg(rating) AS avg_rating
    FROM product_reviews
    WHERE deleted_at IS NULL AND hidden_at IS NULL
    GROUP BY product_id
) s ON s.product_id = p.id
WHERE p.id = $1 AND p.hidden_at IS NULL;

-- name: CreateProduct :one
INSERT INTO products (id, kind, brand, name, image_url, product_url)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: UpdateProduct :execrows
UPDATE products
SET product_url = sqlc.arg(product_url),
    discontinued_at = CASE WHEN sqlc.arg(discontinued)::boolean THEN COALESCE(discontinued_at, now()) ELSE NULL END
WHERE id = sqlc.arg(id) AND hidden_at IS NULL;

-- name: HideProduct :execrows
UPDATE products SET hidden_at = now() WHERE id = $1 AND hidden_at IS NULL;

-- name: ListProductReviews :many
SELECT r.id, r.product_id, r.author_id, r.rating, r.body, r.event, r.level, r.foot_size, r.foot_width, r.usage, r.created_at
FROM product_reviews r
WHERE r.product_id = sqlc.arg(product_id) AND r.deleted_at IS NULL AND r.hidden_at IS NULL
  AND (sqlc.narg(event)::text IS NULL OR r.event = sqlc.narg(event)::text)
  AND (sqlc.narg(level)::text IS NULL OR r.level = sqlc.narg(level)::text)
  AND (sqlc.narg(foot_size_min)::smallint IS NULL OR r.foot_size >= sqlc.narg(foot_size_min)::smallint)
  AND (sqlc.narg(foot_size_max)::smallint IS NULL OR r.foot_size <= sqlc.narg(foot_size_max)::smallint)
  AND (sqlc.narg(foot_width)::text IS NULL OR r.foot_width = sqlc.narg(foot_width)::text)
  AND (sqlc.narg(usage)::text IS NULL OR r.usage = sqlc.narg(usage)::text)
ORDER BY r.created_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: SummarizeProductReviews :one
-- 絞り込みに合うレビューの件数と平均★。
SELECT count(*)::bigint AS review_count, COALESCE(avg(r.rating), 0)::float8 AS avg_rating
FROM product_reviews r
WHERE r.product_id = sqlc.arg(product_id) AND r.deleted_at IS NULL AND r.hidden_at IS NULL
  AND (sqlc.narg(event)::text IS NULL OR r.event = sqlc.narg(event)::text)
  AND (sqlc.narg(level)::text IS NULL OR r.level = sqlc.narg(level)::text)
  AND (sqlc.narg(foot_size_min)::smallint IS NULL OR r.foot_size >= sqlc.narg(foot_size_min)::smallint)
  AND (sqlc.narg(foot_size_max)::smallint IS NULL OR r.foot_size <= sqlc.narg(foot_size_max)::smallint)
  AND (sqlc.narg(foot_width)::text IS NULL OR r.foot_width = sqlc.narg(foot_width)::text)
  AND (sqlc.narg(usage)::text IS NULL OR r.usage = sqlc.narg(usage)::text);

-- name: CreateProductReview :one
INSERT INTO product_reviews (id, product_id, author_id, rating, body, event, level, foot_size, foot_width, usage)
VALUES (
    sqlc.arg(id), sqlc.arg(product_id), sqlc.arg(author_id), sqlc.arg(rating), sqlc.arg(body),
    sqlc.narg(event), sqlc.narg(level), sqlc.narg(foot_size), sqlc.narg(foot_width), sqlc.narg(usage)
)
RETURNING id, product_id, author_id, rating, body, event, level, foot_size, foot_width, usage, created_at;

-- name: SoftDeleteProductReview :execrows
UPDATE product_reviews
SET deleted_at = now()
WHERE id = $1 AND author_id = $2 AND deleted_at IS NULL;

-- name: HideProductReview :execrows
UPDATE product_reviews
SET hidden_at = now()
WHERE id = $1 AND deleted_at IS NULL AND hidden_at IS NULL;

-- name: RankProductsByEvent :many
-- 種目ごとのランキング。その種目の人が書いたレビューだけで平均★を出し、min_reviews 件に満たない商品は外す。
SELECT
    p.id,
    p.kind,
    p.brand,
    p.name,
    p.image_url,
    p.product_url,
    (p.discontinued_at IS NOT NULL)::boolean AS discontinued,
    count(r.id)::bigint AS review_count,
    avg(r.rating)::float8 AS avg_rating
FROM products p
JOIN product_reviews r ON r.product_id = p.id
WHERE p.hidden_at IS NULL
  AND p.kind = sqlc.arg(kind)
  AND r.deleted_at IS NULL AND r.hidden_at IS NULL
  AND r.event = sqlc.arg(event)::text
GROUP BY p.id
HAVING count(r.id) >= sqlc.arg(min_reviews)::bigint
ORDER BY avg(r.rating) DESC, count(r.id) DESC, p.created_at DESC
LIMIT sqlc.arg(row_limit);
