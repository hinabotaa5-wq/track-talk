-- name: ListProducts :many
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
WHERE p.hidden_at IS NULL
  AND p.kind = sqlc.arg(kind)
  AND (sqlc.narg(q)::text IS NULL OR p.name ILIKE '%' || sqlc.narg(q)::text || '%' ESCAPE '\' OR p.brand ILIKE '%' || sqlc.narg(q)::text || '%' ESCAPE '\')
ORDER BY (p.discontinued_at IS NOT NULL) ASC, COALESCE(s.review_count, 0) DESC, COALESCE(s.avg_rating, 0) DESC, p.created_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountProducts :one
SELECT count(*)
FROM products p
WHERE p.hidden_at IS NULL
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
SELECT id, product_id, author_id, rating, body, created_at
FROM product_reviews
WHERE product_id = $1 AND deleted_at IS NULL AND hidden_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountProductReviews :one
SELECT count(*)
FROM product_reviews
WHERE product_id = $1 AND deleted_at IS NULL AND hidden_at IS NULL;

-- name: CreateProductReview :one
INSERT INTO product_reviews (id, product_id, author_id, rating, body)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, product_id, author_id, rating, body, created_at;

-- name: SoftDeleteProductReview :execrows
UPDATE product_reviews
SET deleted_at = now()
WHERE id = $1 AND author_id = $2 AND deleted_at IS NULL;

-- name: HideProductReview :execrows
UPDATE product_reviews
SET hidden_at = now()
WHERE id = $1 AND deleted_at IS NULL AND hidden_at IS NULL;
