
#========================
# DOCKER FOR NATS
#========================

nats/pull:
	docker pull nats:2.12 

## docker/nats/up: starts a docker container with the NATS core
core/up: 
	docker run --name nats-core -p 6969:4222 nats:2.12

## docker/nats/up: starts a docker container with the NATS core
core/down: 
	docker rm nats-core -v 

## core/start: starts a docker container with the NATS core
core/start:
	docker container start nats-core -a

## core/start: stops a docker container with the NATS core
core/stop:
	docker stop nats-core

## ----------------------------------------------------------------------------

## docker/nats/up: starts a docker container with the NATS running on jetstream
stream/up: 
	docker run --name nats-js -p 6969:4222 nats:2.12 -js

## docker/nats/up: remove the container and associated volumes
stream/down: 
	docker rm nats-js -v

## stream/start: starts a docker container with the NATS Jetstream
stream/start:
	docker container start nats-js -a

## stream/start: stops a docker container with the NATS Jetstream
stream/stop:
	docker stop nats-js

