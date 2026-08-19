#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE DATABASE storesdb OWNER eda_dev;
EOSQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname storesdb <<-EOSQL
    GRANT CONNECT ON DATABASE storesdb TO eda_dev, eda_user;

    GRANT USAGE, CREATE ON SCHEMA public TO eda_dev;
    GRANT USAGE ON SCHEMA public TO eda_user;

    ALTER DEFAULT PRIVILEGES FOR ROLE eda_dev IN SCHEMA public
        GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO eda_user;

    ALTER DEFAULT PRIVILEGES FOR ROLE eda_dev IN SCHEMA public
        GRANT USAGE, SELECT ON SEQUENCES TO eda_user;
EOSQL
