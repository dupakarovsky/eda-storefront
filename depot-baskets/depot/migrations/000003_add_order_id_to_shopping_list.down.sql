-- depot/migrations/000002_add_order_id_to_shopping_lists.down.sql
ALTER TABLE shopping_lists DROP COLUMN order_id;
