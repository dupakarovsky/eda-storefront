-- baskets/migrations/000001_create_baskets_table.up.sql
CREATE TABLE baskets (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE basket_items (
    basket_id TEXT NOT NULL REFERENCES baskets (id) ON DELETE CASCADE,
    product_id TEXT NOT NULL,
    quantity INTEGER NOT NULL,
    PRIMARY KEY (basket_id, product_id)
);
