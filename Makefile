## ============================================================================
## --- Container -- 
## ============================================================================

COMPOSE := docker compose

.PHONY: up down logs ps smoke 

up:
	$(COMPOSE) up -d

down:
	# down does NOT tear the the volumes intentionally
	$(COMPOSE) down 

logs:
	$(COMPOSE) logs 

ps:
	$(COMPOSE) ps

nats-box/stores:
	$(COMPOSE) exec nats-box nats stream info stores --server nats://nats:4222 

smoke:
	@echo 'Building application...'
	@mkdir -p ./bin
	GOOS=linux GOARCH=amd64 go build -o=./bin/foundation-smoketest ./foundation-smoketest
	chmod +x ./bin/foundation-smoketest
	./bin/foundation-smoketest

## ============================================================================
## --- Migrations -- 
## ============================================================================

include .postgres.env
export

.PHONY: migrate-up migrate-down
migrate-up:
	@migrate -path 01-stores/migrations \
		-database "postgres://eda_dev:$(EDA_DEV_PASSWORD)@$(MIGRATE_DB_HOST):$(MIGRATE_DB_PORT)/$(MIGRATE_DB_NAME)?sslmode=disable" \
		up

migrate-down:
	@migrate -path 01-stores/migrations \
		-database "postgres://eda_dev:$(EDA_DEV_PASSWORD)@$(MIGRATE_DB_HOST):$(MIGRATE_DB_PORT)/$(MIGRATE_DB_NAME)?sslmode=disable" \
		down 1



## ============================================================================
## --- Stores -- 
## ============================================================================

.PHONY: stores-run
stores-run:
	@echo 'Running stores service...'
	@mkdir -p ./01-stores/bin
	GOOS=linux GOARCH=amd64 go build -o=./01-stores/bin/stores ./01-stores/cmd/service/
	chmod +x ./01-stores/bin/stores
	cd ./01-stores && ./bin/stores

## ============================================================================
## -- Database ---
## ============================================================================

# connect to the storesdb as the edb_dev user
.PHONY: storesdb/dev
storesdb/dev:
	psql -h 127.0.0.1 -p 5433 -d storesdb -U eda_dev

## ============================================================================
## --- Utils -- 
## ============================================================================

# formats markdown file to keep it withint 80 characters long
format:
	mdformat --wrap=80 ./**/README.md


