-- customers/migrations/000001_create_customers_table.up.sql
CREATE TABLE customers (
    id TEXT PRIMARY KEY,
    -- BTRIM removes the longest string that constains a specific character
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    sms_number TEXT NOT NULL CHECK (btrim(sms_number) <> ''),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
