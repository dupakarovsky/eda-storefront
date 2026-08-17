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
