POSTGRES_HOST ?= localhost
POSTGRES_PORT ?= 5432
POSTGRES_DB ?= erx
POSTGRES_USER ?= erx_app
POSTGRES_PASSWORD ?= change-me

DB_URL := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable

.PHONY: run build seed migrate-up migrate-down test test-race coverage lint

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api
	go build -o bin/seed ./cmd/seed

seed:
	go run ./cmd/seed

migrate-up:
	migrate -path ./migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path ./migrations -database "$(DB_URL)" down -all

test:
	go test -v ./...

test-race:
	go test -v -race ./...

coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | grep -E "internal/(cds|pharmacy|pki|qr)|total:"

lint:
	test -z "$$(gofmt -l .)"
	go vet ./...
