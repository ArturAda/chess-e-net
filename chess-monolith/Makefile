-include configs/.env

DB_URL=postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable
DOCKER_DB_URL=postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@postgres:5432/$(POSTGRES_DB)?sslmode=disable

.PHONY: run build-frontend docker-build up down echo-db migrate-create migrate-up migrate-down

run:
	go run cmd/server/main.go

build-frontend:
	cd frontend && npm run build

docker-build:
	docker compose --env-file configs/.env build backend

up:
	docker compose --env-file configs/.env up --build -d

down:
	docker compose --env-file configs/.env down

echo-db:
	@echo "Connecting to DB at: $(DB_URL)"

# Создать новый пустой файл миграции (например: make migrate-create name=init)
migrate-create:
	migrate create -ext sql -dir migrations -seq $(name)

# Применить все миграции к базе данных
migrate-up:
	@if command -v migrate >/dev/null 2>&1; then \
		migrate -path migrations -database "$(DB_URL)" up; \
	else \
		echo "migrate CLI not found. Using Docker migrator..."; \
		docker compose --env-file configs/.env up -d postgres; \
		docker compose --env-file configs/.env run --rm migrator -path=/migrations -database "$(DOCKER_DB_URL)" up; \
	fi

# Откатить последнюю миграцию
migrate-down:
	@if command -v migrate >/dev/null 2>&1; then \
		migrate -path migrations -database "$(DB_URL)" down 1; \
	else \
		echo "migrate CLI not found. Using Docker migrator..."; \
		docker compose --env-file configs/.env up -d postgres; \
		docker compose --env-file configs/.env run --rm migrator -path=/migrations -database "$(DOCKER_DB_URL)" down 1; \
	fi
