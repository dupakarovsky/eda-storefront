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

.PHONY: migrate/up migrate/down
migrate/up:
	@migrate -path 01-stores/migrations \
		-database "postgres://eda_dev:$(EDA_DEV_PASSWORD)@$(MIGRATE_DB_HOST):$(MIGRATE_DB_PORT)/$(MIGRATE_DB_NAME)?sslmode=disable" \
		up

migrate/down:
	@migrate -path 01-stores/migrations \
		-database "postgres://eda_dev:$(EDA_DEV_PASSWORD)@$(MIGRATE_DB_HOST):$(MIGRATE_DB_PORT)/$(MIGRATE_DB_NAME)?sslmode=disable" \
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
	@mkdir -p ./01-stores/bin
	GOOS=linux GOARCH=amd64 go build -o=./01-stores/bin/stores ./01-stores/cmd/service/
	chmod +x ./01-stores/bin/stores
	cd ./01-stores && ./bin/stores

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


