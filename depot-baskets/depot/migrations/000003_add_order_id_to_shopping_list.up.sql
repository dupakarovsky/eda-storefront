-- depot/migrations/000002_add_order_id_to_shopping_lists.up.sql
ALTER TABLE shopping_lists ADD COLUMN order_id TEXT NOT NULL DEFAULT '';
