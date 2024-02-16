SHORT_HASH := $(shell git rev-parse --short HEAD)
CONTAINER_NAME := mygpt

docker-build:
	docker build --build-arg NGROK_AUTHTOKEN=$(NGROK_AUTHTOKEN) -t mycodeinterpreter .

docker-run:
	docker run --rm --name $(CONTAINER_NAME) mycodeinterpreter:$(SHORT_HASH)

docker-attach:
	docker exec -it $(CONTAINER_NAME) /bin/bash

docker-logs:
	docker logs $(CONTAINER_NAME)
