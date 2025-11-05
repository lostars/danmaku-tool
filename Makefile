VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
GO_VERSION := 1.25
BINARY := danmaku
OUTPUT := dist
PROJECT := danmaku-tool
GOOS :=
ARCH :=
LDFLAGS := -ldflags "-X $(PROJECT)/internal/config.Version=$(VERSION) -w -s"
ifeq ($(GOOS),windows)
EXT := .exe
else
EXT :=
endif
BIN := $(BINARY)$(EXT)

.PHONY: build
build:
	@echo "Building locally..."
	@go mod tidy
	@echo "version: $(VERSION)"
	go build $(LDFLAGS) -o $(OUTPUT)/$(BIN) main.go

.PHONY: build-docker
build-docker:
	@echo "Building docker..."
	$(MAKE)	build-artifact ARCH=arm64
	$(MAKE) build-artifact ARCH=amd64
	docker buildx build --platform linux/amd64,linux/arm64 --build-arg OUTPUT=$(OUTPUT) \
	--push -t ghcr.io/lostars/$(PROJECT):dev .

.PHONY: compress
compress:
	@echo "Compressing binary..."
	@cd $(OUTPUT) && tar -czf $(PROJECT)_$(VERSION)_$(GOOS)_$(ARCH).tar.gz $(BIN) && cd ..

.PHONY: build-artifact
build-artifact:
	@echo "Building $(GOOS)-$(ARCH)..."
	@echo "WIN_ARM: $(WIN_ARM)"
	GOOS=$(GOOS) GOARCH=$(ARCH) go build $(LDFLAGS) -o $(OUTPUT)/$(BIN) main.go

.PHONY: release
release: build-artifact compress

.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -rf $(OUTPUT)