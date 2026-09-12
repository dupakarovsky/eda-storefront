-- baskets/migrations/000002_create_catalog_tables.up.sql
CREATE TABLE catalog_stores (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    location TEXT NOT NULL,
    participating BOOLEAN NOT NULL DEFAULT false
);

CREATE TABLE catalog_products (
    id TEXT PRIMARY KEY,
    store_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    sku TEXT NOT NULL,
    price NUMERIC NOT NULL
);
