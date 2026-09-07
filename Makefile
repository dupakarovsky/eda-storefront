## ============================================================================
## FOR DEPLOYMENT
## ============================================================================

## ----------------------------------------------------------------------------
## --- Containers -- 
## ----------------------------------------------------------------------------
COMPOSE := docker compose

.PHONY: up down build logs logs/stores ps
up:
	$(COMPOSE) up -d

# down does NOT tear the the volumes intentionally
down:
	$(COMPOSE) down 

build:
	$(COMPOSE) build

logs:
	$(COMPOSE) logs

logs/stores:
	$(COMPOSE) logs stores

stores/term:
	$(COMPOSE) kill -s SIGTERM stores

ps:
	$(COMPOSE) ps


## ----------------------------------------------------------------------------
## --- Migrations -- 
## ----------------------------------------------------------------------------
include .postgres.env
export

.PHONY: migrate/stores/up migrate/stores/down migrate/baskets/up migrate/baskets/down migrate/depot/up migrate/depot/down 

## --- Stores DB -- 
migrate/stores/up:
	@migrate -path stores/migrations \
		-database "postgres://eda_dev:$(EDA_DEV_PASSWORD)@$(MIGRATE_DB_HOST):$(MIGRATE_DB_PORT)/storesdb?sslmode=disable" \
		up

migrate/stores/down:
	@migrate -path stores/migrations \
		-database "postgres://eda_dev:$(EDA_DEV_PASSWORD)@$(MIGRATE_DB_HOST):$(MIGRATE_DB_PORT)/storesdb?sslmode=disable" \
		down 1
## ----------------------------------------------------------------------------

## --- Baskets DB -- 
migrate/baskets/up:
	@migrate -path baskets/migrations \
		-database "postgres://eda_dev:$(EDA_DEV_PASSWORD)@$(MIGRATE_DB_HOST):$(MIGRATE_DB_PORT)/basketsdb?sslmode=disable" \
		up

migrate/baskets/down:
	@migrate -path baskets/migrations \
		-database "postgres://eda_dev:$(EDA_DEV_PASSWORD)@$(MIGRATE_DB_HOST):$(MIGRATE_DB_PORT)/basketsdb?sslmode=disable" \
		down 1
## ----------------------------------------------------------------------------

## --- Depot DB -- 
migrate/depot/up:
	@migrate -path depot/migrations \
		-database "postgres://eda_dev:$(EDA_DEV_PASSWORD)@$(MIGRATE_DB_HOST):$(MIGRATE_DB_PORT)/depotdb?sslmode=disable" \
		up

migrate/depot/down:
	@migrate -path depot/migrations \
		-database "postgres://eda_dev:$(EDA_DEV_PASSWORD)@$(MIGRATE_DB_HOST):$(MIGRATE_DB_PORT)/depotdb?sslmode=disable" \
		down 1


## ============================================================================
## FOR DEVELOPMENT ONLY
## ============================================================================

## ----------------------------------------------------------------------------
## --- run stores locally -- 
## ----------------------------------------------------------------------------
.PHONY: stores/run foundation/smoke

foundation/smoke:
	@echo 'Building application...'
	@mkdir -p ./bin
	GOOS=linux GOARCH=amd64 go build -o=./bin/foundation-smoketest ./foundation-smoketest
	chmod +x ./bin/foundation-smoketest
	./bin/foundation-smoketest

stores/run:
	@echo 'Running stores service...'
	@mkdir -p ./stores/bin
	GOOS=linux GOARCH=amd64 go build -o=./stores/bin/stores ./stores/cmd/service/
	chmod +x ./stores/bin/stores
	cd ./stores && ./bin/stores

## ----------------------------------------------------------------------------
## -- database access ---
## ----------------------------------------------------------------------------
.PHONY: storesdb/dev
# connect to the storesdb as the edb_dev user
storesdb/dev:
	psql -h 127.0.0.1 -p 5433 -d storesdb -U eda_dev

## ----------------------------------------------------------------------------
## --- uttilities -- 
## ----------------------------------------------------------------------------
.PHONY: format
# formats markdown file to keep it withint 80 characters long
format:
	mdformat --wrap=80 ./**/README.md


