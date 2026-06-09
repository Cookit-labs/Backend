.PHONY: help setup dev db-up db-down build test clean

help:
	@echo "Intent Backend — Available Commands"
	@echo ""
	@echo "  make setup        Install dependencies and setup environment"
	@echo "  make dev          Start server with live reload"
	@echo "  make build        Build server binary"
	@echo "  make db-up        Start PostgreSQL and Redis containers"
	@echo "  make db-down      Stop containers"
	@echo "  make test         Run tests"
	@echo "  make clean        Clean build artifacts"

setup:
	@if [ ! -f .env ]; then cp .env.example .env; fi
	@echo "✓ Environment setup complete"
	@echo "  DATABASE_URL=postgres://intent:intent_dev_password@localhost:5432/intent?sslmode=disable"
	@echo "  REDIS_URL=redis://localhost:6379"

dev:
	@command -v air > /dev/null || go install github.com/cosmtrek/air@latest
	@air -c .air.toml

build:
	@mkdir -p bin
	@go build -o bin/server ./cmd/server

db-up:
	@docker-compose up -d
	@echo "✓ PostgreSQL (5432) and Redis (6379) are running"

db-down:
	@docker-compose down
	@echo "✓ Containers stopped"

db-logs:
	@docker-compose logs -f

test:
	@go test -v ./...

clean:
	@rm -rf bin/
	@go clean
	@echo "✓ Build artifacts cleaned"
