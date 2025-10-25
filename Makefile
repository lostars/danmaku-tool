BINARY_NAME=danmaku
LDFLAGS=-ldflags "-X danmu-tool/internal/config.Version=dev -w -s"

.PHONY: build deps

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) main.go

deps:
	go mod tidy