--- revert
-- depot-baskets/{baskets,depot}/migrations/000004_cap_catalog_price.up.sql
ALTER TABLE catalog_products
    DROP CONSTRAINT catalog_products_price_non_negative,
    ALTER COLUMN price TYPE NUMERIC;
