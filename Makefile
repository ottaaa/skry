BIN := bin/skry
PKG := ./cmd/skry
MODULE := github.com/ottaaa/skry
REVISION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -s -w -X $(MODULE)/version.Revision=$(REVISION)

.PHONY: default ci build run dev dev-seed tidy test test-race cover fmt vet lint lint-install clean

default: test

# CI target: the GitHub Actions runner invokes this.
ci: test-race

build:
	@mkdir -p bin
	go build -ldflags '$(LDFLAGS)' -trimpath -o $(BIN) $(PKG)

run:
	go run $(PKG) $(ARGS)

# Dev smoke-test against the bundled sample repo. Override ARGS to point at
# any other path, e.g. `make dev ARGS=~/code/myrepo`. The sample repo is
# re-seeded automatically before each launch when ARGS is unset.
dev: build
	@if [ -z "$(ARGS)" ]; then $(MAKE) dev-seed; fi
	./$(BIN) $(if $(ARGS),$(ARGS),testdata/sample-repo)

# Re-create the sample repo from scratch. Safe to run repeatedly.
dev-seed:
	bash testdata/sample-repo/seed.sh

tidy:
	go mod tidy

test:
	go test ./...

test-race:
	go test -race -covermode=atomic -coverprofile=coverage.out ./...

cover: test-race
	go tool cover -html=coverage.out -o coverage.html
	@echo "open coverage.html"

fmt:
	go fmt ./...

vet:
	go vet ./...

# Requires golangci-lint (v2+). Install with `make lint-install` or via
# Homebrew: `brew install golangci-lint`.
lint:
	golangci-lint run ./...

lint-install:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

clean:
	rm -rf bin coverage.out coverage.html dist
