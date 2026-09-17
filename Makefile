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

# build all services from their Dockerfiles
build:
	$(COMPOSE) build

# logs of services
logs:
	$(COMPOSE) logs

logs/stores:
	$(COMPOSE) logs stores

logs/baskets:
	$(COMPOSE) logs baskets

logs/depot:
	$(COMPOSE) logs depot

kill/stores:
	$(COMPOSE) kill -s SIGTERM stores

kill/baskets:
	$(COMPOSE) kill -s SIGTERM baskets

kill/depot:
	$(COMPOSE) kill -s SIGTERM depot 

ps:
	$(COMPOSE) ps

exec/pg:
	$(COMPOSE) exec postgres psql -h 127.0.0.1 -U edasuper -d edadb

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
	@migrate -path depot-baskets/baskets/migrations \
		-database "postgres://eda_dev:$(EDA_DEV_PASSWORD)@$(MIGRATE_DB_HOST):$(MIGRATE_DB_PORT)/basketsdb?sslmode=disable" \
		up

migrate/baskets/down:
	@migrate -path depot-baskets/baskets/migrations \
		-database "postgres://eda_dev:$(EDA_DEV_PASSWORD)@$(MIGRATE_DB_HOST):$(MIGRATE_DB_PORT)/basketsdb?sslmode=disable" \
		down 1
## ----------------------------------------------------------------------------

## --- Depot DB -- 
migrate/depot/up:
	@migrate -path depot-baskets/depot/migrations \
		-database "postgres://eda_dev:$(EDA_DEV_PASSWORD)@$(MIGRATE_DB_HOST):$(MIGRATE_DB_PORT)/depotdb?sslmode=disable" \
		up

migrate/depot/down:
	@migrate -path depot-baskets/depot/migrations \
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

baskets/run:
	@echo 'Running baskets service...'
	@mkdir -p ./depot-baskets/baskets/bin
	GOOS=linux GOARCH=amd64 go build -o=./depot-baskets/baskets/bin/baskets ./depot-baskets/baskets/cmd/service/
	chmod +x ./depot-baskets/baskets/bin/baskets
	cd ./depot-baskets/baskets && ./bin/baskets

depot/run:
	@echo 'Running depot service...'
	@mkdir -p ./depot-baskets/depot/bin
	GOOS=linux GOARCH=amd64 go build -o=./depot-baskets/depot/bin/depot ./depot-baskets/depot/cmd/service/
	chmod +x ./depot-baskets/depot/bin/depot
	cd ./depot-baskets/depot && ./bin/depot


## ----------------------------------------------------------------------------
## -- database access ---
## ----------------------------------------------------------------------------
.PHONY: storesdb/dev
# connect to the storesdb as the edb_dev user
storesdb/dev:
	psql -h 127.0.0.1 -p 5433 -d storesdb -U eda_dev

.PHONY: basketsdb/dev
# connect to the storesdb as the edb_dev user
basketsdb/dev:
	psql -h 127.0.0.1 -p 5433 -d basketsdb -U eda_dev

.PHONY: depotdb/dev
# connect to the storesdb as the edb_dev user
depotdb/dev:
	psql -h 127.0.0.1 -p 5433 -d depotdb -U eda_dev

## ----------------------------------------------------------------------------
## -- nats inspection ---
## ----------------------------------------------------------------------------
.PHONY: nats/baskets
nats/baskets:
	# docker run --rm --network eda-storefront_eda-net natsio/nats-box:latest  nats --server nats://nats:4222 stream info baskets
	docker run --rm --network eda-storefront_eda-net natsio/nats-box:latest  nats --server nats://nats:4222 stream info baskets

## ----------------------------------------------------------------------------
## --- uttilities -- 
## ----------------------------------------------------------------------------
.PHONY: format
# formats markdown file to keep it withint 80 characters long
format:
	mdformat --wrap=80 ./**/*.md


