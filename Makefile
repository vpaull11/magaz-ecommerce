.PHONY: run run-payment build tidy lint test docker-up docker-down

run:
	go run ./cmd/server

run-payment:
	go run ./cmd/payment

build:
	go build -o bin/server ./cmd/server
	go build -o bin/payment ./cmd/payment

tidy:
	go mod tidy

lint:
	go vet ./...

test:
	go test ./... -v -count=1

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

.env:
	cp .env.example .env
	@echo "Created .env — please fill in your secrets"
