CREATE TABLE products (
    id uuid PRIMARY KEY,
    kind text NOT NULL CHECK (kind IN ('shoe', 'spike')),
    brand text NOT NULL CHECK (char_length(brand) BETWEEN 1 AND 60),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    image_url text NOT NULL DEFAULT '' CHECK (char_length(image_url) <= 500),
    hidden_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX products_brand_name_idx ON products (lower(brand), lower(name));
CREATE INDEX products_kind_idx ON products (kind) WHERE hidden_at IS NULL;

CREATE TABLE product_reviews (
    id uuid PRIMARY KEY,
    product_id uuid NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    author_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    rating smallint NOT NULL CHECK (rating BETWEEN 1 AND 5),
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
    created_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    hidden_at timestamptz
);

CREATE UNIQUE INDEX product_reviews_one_per_user_idx
    ON product_reviews (product_id, author_id)
    WHERE deleted_at IS NULL;
CREATE INDEX product_reviews_product_created_idx ON product_reviews (product_id, created_at DESC);

ALTER TABLE products ENABLE ROW LEVEL SECURITY;
ALTER TABLE product_reviews ENABLE ROW LEVEL SECURITY;

INSERT INTO products (id, kind, brand, name) VALUES
    (gen_random_uuid(), 'shoe', 'Nike', 'Alphafly 3'),
    (gen_random_uuid(), 'shoe', 'Nike', 'Vaporfly 3'),
    (gen_random_uuid(), 'shoe', 'adidas', 'Adizero Adios Pro 3'),
    (gen_random_uuid(), 'shoe', 'ASICS', 'METASPEED SKY'),
    (gen_random_uuid(), 'shoe', 'HOKA', 'Cielo X1'),
    (gen_random_uuid(), 'shoe', 'PUMA', 'Fast-R NITRO Elite 2'),
    (gen_random_uuid(), 'spike', 'Nike', 'Dragonfly 2'),
    (gen_random_uuid(), 'spike', 'Nike', 'Maxfly 2'),
    (gen_random_uuid(), 'spike', 'adidas', 'Adizero Prime SP 3'),
    (gen_random_uuid(), 'spike', 'ASICS', 'METASPRINT 2'),
    (gen_random_uuid(), 'spike', 'ASICS', 'HYPER SPRINT 9'),
    (gen_random_uuid(), 'spike', 'PUMA', 'evoSPEED Long Distance Nitro Elite 2');
