.PHONY: help all build-server build-client build clean install test

# Help
help:
	@echo "Secure MQTT Shell - Makefile Commands"
	@echo "======================================"
	@echo ""
	@echo "Building:"
	@echo "  make build              - Build both server and client"
	@echo "  make build-server       - Build server only"
	@echo "  make build-client       - Build client only"
	@echo "  make build-server-linux - Build server for Linux"
	@echo "  make build-all          - Build for all platforms"
	@echo ""
	@echo "Development:"
	@echo "  make run-server         - Run server locally"
	@echo "  make run-client         - Run client locally"
	@echo "  make test               - Run tests"
	@echo "  make install            - Install dependencies"
	@echo ""
	@echo "Setup:"
	@echo "  make setup              - Create .env from .env.example"
	@echo "  make clean              - Remove build artifacts"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-up          - Start Docker containers"
	@echo "  make docker-down        - Stop Docker containers"
	@echo "  make docker-logs        - View Docker logs"

# Default target
all: build

# Build both server and client
build: build-server build-client

# Build server
build-server:
	@echo "Building server..."
	cd server && go mod tidy && go build -o ../bin/mqtt-shell-server

# Build client
build-client:
	@echo "Building client..."
	cd client && go mod tidy && go build -o ../bin/mqtt-shell-client

# Build server for Linux (for deployment remote servers)
build-server-linux:
	@echo "Building server for Linux..."
	cd server && GOOS=linux GOARCH=amd64 go build -o ../bin/mqtt-shell-server-linux

# Build for all platforms
build-all:
	@echo "Building for all platforms..."
	# Server
	cd server && GOOS=linux GOARCH=amd64 go build -o ../bin/mqtt-shell-server-linux-amd64
	cd server && GOOS=linux GOARCH=arm64 go build -o ../bin/mqtt-shell-server-linux-arm64
	cd server && GOOS=darwin GOARCH=amd64 go build -o ../bin/mqtt-shell-server-darwin-amd64
	cd server && GOOS=darwin GOARCH=arm64 go build -o ../bin/mqtt-shell-server-darwin-arm64
	cd server && GOOS=windows GOARCH=amd64 go build -o ../bin/mqtt-shell-server-windows-amd64.exe
	# Client
	cd client && GOOS=linux GOARCH=amd64 go build -o ../bin/mqtt-shell-client-linux-amd64
	cd client && GOOS=linux GOARCH=arm64 go build -o ../bin/mqtt-shell-client-linux-arm64
	cd client && GOOS=darwin GOARCH=amd64 go build -o ../bin/mqtt-shell-client-darwin-amd64
	cd client && GOOS=darwin GOARCH=arm64 go build -o ../bin/mqtt-shell-client-darwin-arm64
	cd client && GOOS=windows GOARCH=amd64 go build -o ../bin/mqtt-shell-client-windows-amd64.exe

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	cd server && go clean
	cd client && go clean

# Install dependencies
install:
	@echo "Installing dependencies..."
	cd server && go mod download
	cd client && go mod download

# Run tests
test:
	@echo "Running tests..."
	cd server && go test -v ./...
	cd client && go test -v ./...

# Run server locally
run-server:
	@echo "Running server..."
	cd server && go run .

# Run client locally
run-client:
	@echo "Running client..."
	cd client && go run .

# Setup environment
setup:
	@echo "Setting up environment..."
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "Created .env file from .env.example"; \
		echo "Please edit .env and set EXEC_KEY to a strong password!"; \
	else \
		echo ".env already exists, skipping..."; \
	fi

# Docker compose commands
docker-up:
	@echo "Starting Docker containers..."
	docker-compose up -d

docker-down:
	@echo "Stopping Docker containers..."
	docker-compose down

docker-logs:
	@echo "Showing Docker logs..."
	docker-compose logs -f

