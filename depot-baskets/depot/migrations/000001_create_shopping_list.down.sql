-- depot/migrations/000001_create_shopping_lists.up.sql
CREATE TABLE shopping_lists (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    initiated_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- shopping_list_stops references the shopping_lists table
CREATE TABLE shopping_list_stops (
    shopping_list_id TEXT NOT NULL REFERENCES shopping_lists(id) ON DELETE CASCADE,
    store_id TEXT NOT NULL,
    store_name TEXT NOT NULL,
    store_location TEXT NOT NULL,
    PRIMARY KEY (shopping_list_id, store_id)
);

-- shopping_list_items references the shopping_list_stops table by it's composite key
-- (shopping_list_id + store_id)
CREATE TABLE shopping_list_items (
    shopping_list_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    product_id TEXT NOT NULL,
    quantity INTEGER NOT NULL,
    PRIMARY KEY (shopping_list_id, store_id, product_id),
    FOREIGN KEY (shopping_list_id, store_id)
        REFERENCES shopping_list_stops (shopping_list_id, store_id) ON DELETE CASCADE
);
