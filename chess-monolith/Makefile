.PHONY: run up down

run:
	go run cmd/server/main.go

up:
	docker compose --env-file configs/.env up -d

down:
	docker compose --env-file configs/.env down