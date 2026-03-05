.PHONY: build build-linux build-frontend deploy run test lint migrate-up migrate-down migrate-status migrate-create sqlc docker-up docker-down

# Build
build:
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker
	go build -o bin/migrate ./cmd/migrate

build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/api ./cmd/api
	GOOS=linux GOARCH=amd64 go build -o bin/worker ./cmd/worker
	GOOS=linux GOARCH=amd64 go build -o bin/migrate ./cmd/migrate

build-frontend:
	cd frontend && npm run build

# Run
run:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

# Test
test:
	go test ./... -v -count=1

test-cover:
	go test ./... -coverprofile=coverage.out -count=1
	go tool cover -html=coverage.out -o coverage.html

# Lint
lint:
	golangci-lint run ./...

vet:
	go vet ./...

# Database
migrate-up:
	go run ./cmd/migrate -cmd up

migrate-down:
	go run ./cmd/migrate -cmd down

migrate-status:
	go run ./cmd/migrate -cmd status

migrate-create:
	@read -p "Migration name: " name; \
	go run ./cmd/migrate -cmd create $$name

# sqlc
sqlc:
	sqlc generate

# Docker
docker-up:
	docker compose up -d

docker-down:
	docker compose down

# Dev dependencies
deps:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest

# Tidy
tidy:
	go mod tidy
