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
	go run ./foundation-smoketest

