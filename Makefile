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


smoke:
	@echo 'Building application...'
	@mkdir -p ./bin
	GOOS=linux GOARCH=amd64 go build -o=./bin/foundation-smoketest ./foundation-smoketest
	chmod +x ./bin/foundation-smoketest
	./bin/foundation-smoketest

# -----
format:
	mdformat --wrap=80 ./**/README.md

## --- Migrations 

# Makefile
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
