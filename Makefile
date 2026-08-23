.PHONY: all build clean fmt run setup-hooks test tidy vet

MODULE_NAME := asset-transaction-service
BINARY_NAME := $(MODULE_NAME)-binary
BUILD_DIR := bin

all: tidy build

build:
	@echo "-> Building Go binary for Linux..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux go build -trimpath -o $(BUILD_DIR)/$(BINARY_NAME) ./main.go

clean:
	@echo "-> Removing compiled binary..."
	rm -rf $(BUILD_DIR)

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

run:
	go run ./main.go

setup-hooks:
	./scripts/setup-hooks.sh

test:
	go test ./...

tidy:
	go mod tidy

vet:
	go vet ./...