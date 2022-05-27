.PHONY: docker-build docker-up go-build go-test

SHELL := /bin/bash

docker-build:
	docker compose build

docker-up:
	docker compose up

go-build:
	go build -o indexer oasis-indexer/main.go

go-test:
	go test -v ./...
