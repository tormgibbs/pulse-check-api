include .env
export

.PHONY: run migrate-up migrate-down migrate-create setup docker-up docker-down docker-migrate

run:
	go run ./cmd/api

migrate-up:
	migrate -path internal/db/migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path internal/db/migrations -database "$(DATABASE_URL)" down

migrate-create:
	migrate create -ext sql -dir internal/db/migrations -seq $(name)

setup:
	cp .env.example .env
	direnv allow .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-migrate:
	migrate -path internal/db/migrations -database "postgres://pulse:pulse@localhost:5432/pulsecheck?sslmode=disable" up
