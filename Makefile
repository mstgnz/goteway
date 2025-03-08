.PHONY: build run clean test docker-build docker-run docker-stop docker-clean docker-compose-up docker-compose-down

# Build the application
build:
	go build -o goteway ./cmd/main.go

# Run the application
run:
	go run ./cmd/main.go

# Clean the binary
clean:
	rm -f goteway

# Run tests
test:
	go test -v ./...

# Install dependencies
deps:
	go mod download

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Build and run
dev: build
	./goteway

# Docker build
docker-build:
	docker build -t goteway:latest .

# Docker run
docker-run:
	docker run -p 8080:8080 -v $(PWD)/config.json:/app/config.json --name goteway goteway:latest

# Docker stop
docker-stop:
	docker stop goteway || true
	docker rm goteway || true

# Docker clean
docker-clean: docker-stop
	docker rmi goteway:latest || true

# Docker compose up
docker-compose-up:
	docker-compose up -d

# Docker compose down
docker-compose-down:
	docker-compose down

# Default target
all: clean build 