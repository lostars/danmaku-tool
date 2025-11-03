VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
GO_VERSION := 1.25
BINARY := danmaku
OUTPUT := dist
PROJECT := danmaku-tool
GOOS :=
ARCH :=
CGO_ENABLED := 1
WIN_ARM := $(and $(filter windows,$(GOOS)), $(filter arm64,$(ARCH)))
ifeq ($(GOOS),linux)
LDFLAGS := -ldflags '-X $(PROJECT)/internal/config.Version=$(VERSION) -w -s -extldflags "-static"'
else
LDFLAGS := -ldflags '-X $(PROJECT)/internal/config.Version=$(VERSION) -w -s'
endif
GOJIEBA_MOD_DIR := $(shell go list -m -f '{{.Dir}}' github.com/yanyiwu/gojieba | tr '\\' '/')
ifeq ($(GOOS),windows)
EXT := .exe
else
EXT :=
endif
BIN := $(BINARY)$(EXT)

.PHONY: build
build: copy-dict
	@echo "Building locally..."
	@go mod tidy
	@echo "version: $(VERSION)"
	CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) -o $(OUTPUT)/$(BIN) main.go

.PHONY: build-docker
build-docker:
	@echo "Building docker..."
	docker buildx build --platform linux/amd64,linux/arm64 -t $(PROJECT):$(VERSION) -t $(PROJECT):latest .

.PHONY: compress
compress:
	@echo "Compressing binary..."
	@cd $(OUTPUT) && tar -czf $(PROJECT)_$(VERSION)_$(GOOS)_$(ARCH).tar.gz $(BIN) && cd ..

.PHONY: copy-dict
copy-dict:
	@mkdir -p $(OUTPUT)/dict
	@cp -f $(GOJIEBA_MOD_DIR)/deps/cppjieba/dict/*.utf8 $(OUTPUT)/dict/ || true

.PHONY: build-artifact
build-artifact: copy-dict
	@echo "Building $(GOOS)-$(ARCH)..."
	go mod download
	@echo "WIN_ARM: $(WIN_ARM)"
ifneq ($(WIN_ARM),)
	docker run --rm \
      -e CGO_ENABLED=$(CGO_ENABLED) \
      -e GOOS=$(GOOS) \
      -e GOARCH=$(ARCH) \
      -v "$$(PWD):/app" \
      -w /app \
      x1unix/go-mingw:1.25 \
      /bin/bash -c "go build $(LDFLAGS) -o $(OUTPUT)/$(BIN) main.go"
else
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(ARCH) go build $(LDFLAGS) -o $(OUTPUT)/$(BIN) main.go
endif

.PHONY: release
release: build-artifact compress

.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -rf $(OUTPUT)