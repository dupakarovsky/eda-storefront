#!/bin/bash
set -e

# Connect to postgres in the container as "edasuper"
# Create the basketsdb database asigning "eda_dev"" as the owner
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE DATABASE basketsdb OWNER eda_dev;
EOSQL

# Connect to postgres in the container as "edasuper"
# Grant connect privileges to both "eda_dev" and "ead_user" to the new database
# Grant the create command on the public schema only to "eda_dev".
# "eda_user" has only table usage privileges
# Extend those same provileges to every new table that is created
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname basketsdb <<-EOSQL
    GRANT CONNECT ON DATABASE basketsdb TO eda_dev, eda_user;

    GRANT USAGE, CREATE ON SCHEMA public TO eda_dev;
    GRANT USAGE ON SCHEMA public TO eda_user;

    ALTER DEFAULT PRIVILEGES FOR ROLE eda_dev IN SCHEMA public
        GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO eda_user;

    ALTER DEFAULT PRIVILEGES FOR ROLE eda_dev IN SCHEMA public
        GRANT USAGE, SELECT ON SEQUENCES TO eda_user;
EOSQL
