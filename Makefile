BIN := bin/skry
PKG := ./cmd/skry
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build run tidy test fmt vet clean

build:
	@mkdir -p bin
	go build -ldflags '$(LDFLAGS)' -o $(BIN) $(PKG)

run:
	go run $(PKG) $(ARGS)

tidy:
	go mod tidy

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

clean:
	rm -rf bin
