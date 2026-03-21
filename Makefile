BINARY  := lite-llm
MODULE  := github.com/magifd2/lite-llm
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

# Build output directories
BIN_DIR  := bin
DIST_DIR := dist

# Cross-compilation targets
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64

.PHONY: all build test vet lint check setup build-all clean

## all: default target — build the binary
all: build

## build: compile the binary for the current platform
build:
	@mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY) .

## test: run all unit tests
test:
	go test ./...

## vet: run go vet
vet:
	go vet ./...

## lint: run golangci-lint (must be installed)
lint:
	golangci-lint run ./...

## check: full quality gate — vet + lint + test + build + security scan
check: vet lint test build
	@echo "--- security scan ---"
	govulncheck ./...
	@echo "--- all checks passed ---"

## setup: install git hooks
setup:
	@echo "Installing git hooks..."
	@cp scripts/hooks/pre-commit .git/hooks/pre-commit
	@cp scripts/hooks/pre-push .git/hooks/pre-push
	@chmod +x .git/hooks/pre-commit .git/hooks/pre-push
	@echo "Git hooks installed."

## build-all: cross-compile for all target platforms
build-all:
	@mkdir -p $(DIST_DIR)
	$(foreach PLATFORM,$(PLATFORMS), \
		$(eval GOOS=$(word 1,$(subst /, ,$(PLATFORM)))) \
		$(eval GOARCH=$(word 2,$(subst /, ,$(PLATFORM)))) \
		$(eval EXT=$(if $(filter windows,$(GOOS)),.exe,)) \
		GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) \
			-o $(DIST_DIR)/$(BINARY)-$(GOOS)-$(GOARCH)$(EXT) . ; \
	)
	@echo "Cross-compiled binaries in $(DIST_DIR)/"

## clean: remove build artifacts
clean:
	@rm -rf $(BIN_DIR) $(DIST_DIR)
