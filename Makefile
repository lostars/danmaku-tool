VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
GO_VERSION := 1.25
BINARY := danmaku
OUTPUT := dist
PROJECT := danmaku-tool
LDFLAGS := -ldflags '-X $(PROJECT)/internal/config.Version=$(VERSION) -w -s'
LDFLAGS_LINUX := -ldflags '-X $(PROJECT)/internal/config.Version=$(VERSION) -w -s -extldflags "-static"'
GOOS :=
ARCH :=
GOJIEBA_MOD_DIR := $(shell go list -m -f '{{.Dir}}' github.com/yanyiwu/gojieba | tr '\\' '/')
ifeq ($(GOOS),windows)
EXT := .exe
else
EXT :=
endif
BIN := $(BINARY)$(EXT)

.PHONY: build
build:
	@echo "--- Building local binary ($(OUTPUT)/$(BIN)) with CGO=$(CGO_STATUS) ---"
	go mod tidy
	@echo "$(VERSION)"
	CGO_ENABLED=1 go build $(LDFLAGS) -o bin/$(BIN) main.go

.PHONY: build-docker
build-docker:
	@echo "Building docker..."
	docker buildx build -t $(PROJECT):$(VERSION) -t $(PROJECT):latest .

.PHONY: compress
compress:
	@cd $(OUTPUT) && tar -czf $(PROJECT)_$(VERSION)_$(GOOS)_$(ARCH).tar.gz $(BIN) && cd ..

.PHONY: release
release:
	@echo "Building $(GOOS)-$(ARCH)..."
	go mod tidy
	go mod download
	mkdir -p $(OUTPUT)/dict
	cp $(GOJIEBA_MOD_DIR)/deps/cppjieba/dict/* $(OUTPUT)/dict/ || true
ifeq ($(GOOS),linux)
	CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(ARCH) go build $(LDFLAGS_LINUX) -o $(OUTPUT)/$(BIN) main.go
else
	CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(ARCH) go build $(LDFLAGS) -o $(OUTPUT)/$(BIN) main.go
endif

.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -rf $(OUTPUT)

.PHONY: build-win-arm64
build-win-arm64:
	go mod tidy
	go mod download
	cp $(GOJIEBA_MOD_DIR)/deps/cppjieba/dict/* $(OUTPUT)/dict/ || true
	@bash -c ' \
		mkdir -p "$(OUTPUT)"; \
		docker run --rm \
			-e CGO_ENABLED=1 \
			-e GOOS=windows \
			-e GOARCH=arm64 \
			-v "$$(pwd):/go/src/app" \
			-w /go/src/app \
			x1unix/go-mingw:1.25 \
			go build $(LDFLAGS) -o $(OUTPUT)/$(BIN) main.go; \
	'