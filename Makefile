include .env
export

.PHONY: run migrate-up migrate-down migrate-create setup

run:
	go run ./cmd/server

migrate-up:
	migrate -path internal/db/migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path internal/db/migrations -database "$(DATABASE_URL)" down

migrate-create:
	migrate create -ext sql -dir internal/db/migrations -seq $(name)

setup:
	cp .env.example .env
	direnv allow .
