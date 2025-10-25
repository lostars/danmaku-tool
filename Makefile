BINARY_NAME=danmaku
LDFLAGS=-ldflags "-X danmu-tool/internal/config.Version=dev -w -s"

.PHONY: build deps

build:
# 	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)_linux_arm64 main.go
# 	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)_linux_amd64 main.go
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) main.go

deps:
	go mod tidy