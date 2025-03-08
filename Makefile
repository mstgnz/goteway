.PHONY: build run clean test

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

# Default target
all: clean build 