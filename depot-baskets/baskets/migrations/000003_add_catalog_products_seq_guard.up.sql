-- baskets/migrations/000003_add_catalog_products_seq_guard.up.sql
ALTER TABLE catalog_products
    ADD COLUMN last_applied_seq BIGINT NOT NULL DEFAULT 0;
