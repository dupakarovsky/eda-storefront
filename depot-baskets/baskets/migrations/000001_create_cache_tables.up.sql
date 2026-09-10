-- baskets/migrations/000001_create_cache_tables.up.sql
CREATE TABLE store_cache (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    location TEXT NOT NULL,
    participating BOOLEAN NOT NULL DEFAULT false,
    last_seq BIGINT NOT NULL
);

CREATE TABLE product_cache (
    id TEXT PRIMARY KEY,
    store_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    sku TEXT NOT NULL,
    price NUMERIC(12, 2) NOT NULL DEFAULT 0,
    last_seq BIGINT NOT NULL
);

CREATE INDEX idx_product_cache_store_id ON product_cache (store_id);
