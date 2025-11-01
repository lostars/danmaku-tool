VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
GO_VERSION := 1.25
BINARY := danmaku
OUTPUT := dist
PROJECT := danmaku-tool
LDFLAGS := -ldflags "-X $(PROJECT)/internal/config.Version=$(VERSION) -w -s"
GOOS :=
ARCH :=

.PHONY: build
build:
	@echo "--- Building local binary ($(OUTPUT)/$(BINARY)) with CGO=$(CGO_STATUS) ---"
	go mod tidy
	@echo "$(VERSION)"
	CGO_ENABLED=1 go build $(LDFLAGS) -o bin/$(BINARY) main.go

.PHONY: build-docker
build-docker:
	@echo "Building docker..."
	docker buildx build -t $(PROJECT):$(VERSION) -t $(PROJECT):latest .

.PHONY: compress
compress:
	@cd $(OUTPUT) && tar -czf $(PROJECT)_$(VERSION)_$(GOOS)_$(ARCH).tar.gz $(BINARY) && cd ..

.PHONY: release
release:
	@echo "Building $(GOOS)-$(ARCH)..."
	go mod tidy
	go mod download
	CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(ARCH) go build $(LDFLAGS) -o $(OUTPUT)/$(BINARY) main.go

.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -rf $(OUTPUT)