# Minimal dev Makefile. CI / release builds go through goreleaser
# (see .goreleaser.yaml); this file is for local iteration.

BINARY  := kennel
MODULE  := github.com/kentny/kennel
PKG     := ./cmd/$(BINARY)
PREFIX  ?= $(HOME)/.local

# Version info baked in via ldflags so `kennel version` shows something
# useful even when built outside goreleaser. `git describe` falls back to
# `dev` when there are no tags yet.
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.Date=$(DATE)

.PHONY: help build install test lint fmt vet clean

help:
	@awk 'BEGIN{FS=":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  %-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the binary into ./$(BINARY)
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

install: build ## Install the binary to $(PREFIX)/bin (default: ~/.local/bin)
	install -d $(PREFIX)/bin
	install $(BINARY) $(PREFIX)/bin/$(BINARY)
	@echo "installed $(PREFIX)/bin/$(BINARY) (version $(VERSION))"

test: ## Run unit tests
	go test ./...

lint: vet ## Run gofmt check + go vet
	@test -z "$$(gofmt -l . | tee /dev/stderr)" || { echo "run: gofmt -w ."; exit 1; }

fmt: ## Format all Go sources
	gofmt -w .

vet: ## Run go vet
	go vet ./...

clean: ## Remove build artifacts
	rm -f $(BINARY)
	rm -rf dist/
