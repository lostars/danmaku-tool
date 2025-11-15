VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
GO_VERSION := 1.25
BINARY := danmaku
OUTPUT := dist
PROJECT := danmaku-tool
TARGETS := windows/arm64 windows/amd64 linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
DOCKER_TARGETS := linux/amd64 linux/arm64
LDFLAGS := -ldflags "-X $(PROJECT)/internal/config.Version=$(VERSION) -w -s" -trimpath
CGO_ENABLED := 0

.PHONY: build
build:
	@echo "Building locally..."
	@go mod tidy
	@echo "version: $(VERSION)"
	CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) -o $(OUTPUT)/$(BINARY) main.go

.PHONY: docker
docker: clean docker-binary
	docker buildx build --platform linux/amd64,linux/arm64 --build-arg OUTPUT=$(OUTPUT) \
	--push -t ghcr.io/lostars/$(PROJECT):dev .

.PHONY: docker-binary
docker-binary:
	@for combo in $(DOCKER_TARGETS); do \
		GOOS=$$(echo $$combo | cut -d/ -f1); \
		ARCH=$$(echo $$combo | cut -d/ -f2); \
		mkdir -p $(OUTPUT)/$${GOOS}/$${ARCH}; \
		echo "Building $${GOOS}/$${ARCH}..."; \
		CGO_ENABLED=$(CGO_ENABLED) GOOS=$$GOOS GOARCH=$$ARCH go build $(LDFLAGS) -o $(OUTPUT)/$${GOOS}/$${ARCH}/$(BINARY) main.go; \
	done

.PHONY: artifact
artifact: clean
	@for combo in $(TARGETS); do \
		GOOS=$$(echo $$combo | cut -d/ -f1); \
		ARCH=$$(echo $$combo | cut -d/ -f2); \
		mkdir -p $(OUTPUT)/$${GOOS}/$${ARCH}; \
		echo "Building $${GOOS}/$${ARCH}..."; \
		if [ "$${GOOS}" = "windows" ]; then \
			EXT=".exe"; \
		else \
			EXT=""; \
		fi; \
		BIN="$(BINARY)$${EXT}"; \
		CGO_ENABLED=$(CGO_ENABLED) GOOS=$$GOOS GOARCH=$$ARCH go build $(LDFLAGS) -o $(OUTPUT)/$${GOOS}/$${ARCH}/$${BIN} main.go; \
		tar -czf $(OUTPUT)/$(PROJECT)_$(VERSION)_$${GOOS}_$${ARCH}.tar.gz -C $(OUTPUT)/$${GOOS}/$${ARCH} $${BIN}; \
	done

.PHONY: release
release: artifact

.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -rf $(OUTPUT)