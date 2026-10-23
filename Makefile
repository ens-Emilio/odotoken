.PHONY: build test lint clean run

BINARY_NAME=odotoken
BUILD_DIR=./bin

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/odotoken

test:
	@echo "Running tests..."
	go test -race -coverprofile=coverage.out -v ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	@echo "Running linter..."
	golangci-lint run

clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

run: build
	$(BUILD_DIR)/$(BINARY_NAME) start

deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest