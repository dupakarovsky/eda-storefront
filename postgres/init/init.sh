#!/bin/bash
set -e

PGCONF="/var/lib/postgresql/data/postgresql.conf"

if ! grep -q "^include_dir" "$PGCONF"; then
    echo "include_dir = '/etc/postgresql/conf.d'" >>"$PGCONF"
    echo "LOG: include_dir added to postgresql.conf"
fi
