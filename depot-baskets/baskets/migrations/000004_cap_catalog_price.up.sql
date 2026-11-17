-- depot-baskets/{baskets,depot}/migrations/000004_cap_catalog_price.up.sql
ALTER TABLE catalog_products
    ALTER COLUMN price TYPE NUMERIC(10,2) USING round(price, 2),
    ADD CONSTRAINT catalog_products_price_non_negative CHECK ( price >= 0);
