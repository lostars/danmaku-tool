BINARY_NAME=danmaku
LDFLAGS=-ldflags "-X main.Version=dev -w -s"

.PHONY: build deps

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) cmd/main.go

deps:
	go mod tidy