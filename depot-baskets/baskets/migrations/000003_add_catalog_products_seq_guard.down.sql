-- baskets/migrations/000003_add_catalog_products_seq_guard.down.sql
ALTER TABLE catalog_products
    DROP COLUMN last_applied_seq;
