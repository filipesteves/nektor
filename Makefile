BINARY     := nektor
MODULE     := github.com/filipesteves/nektor
BUILD_DIR  := dist

VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -ldflags "-s -w -X github.com/filipesteves/nektor/cmd.Version=$(VERSION)"

LOCAL_BIN  := $(HOME)/.local/bin

.PHONY: all build build-linux-arm64 install clean tidy

all: build

## build: Compile the binary for the current platform.
build:
	go build $(LDFLAGS) -o $(BINARY) .

## build-linux-arm64: Cross-compile a static binary for Linux ARM64 (e.g. Raspberry Pi).
build-linux-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64 .
	@echo "Built $(BUILD_DIR)/$(BINARY)-linux-arm64"

## install: Build and copy the binary to ~/.local/bin.
install: build
	@mkdir -p $(LOCAL_BIN)
	install -m 0755 $(BINARY) $(LOCAL_BIN)/$(BINARY)
	@echo "Installed to $(LOCAL_BIN)/$(BINARY)"

## clean: Remove build artifacts.
clean:
	rm -f $(BINARY)
	rm -rf $(BUILD_DIR)

## tidy: Tidy Go module dependencies.
tidy:
	go mod tidy

## help: Print this help message.
help:
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/^## /  /'
