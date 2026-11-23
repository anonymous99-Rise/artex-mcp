BINARY  := bin/artex-mcp
VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build test vet fmt tidy routes clean install

all: build

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/artex-mcp

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './bin/*')

tidy:
	go mod tidy

# Regenerate the embedded API route catalog from a local ARTEX checkout.
# Usage: make routes ARTEX_SRC=/path/to/ARTEX-God
ARTEX_SRC ?= ../ARTEX-God
routes:
	python3 -I scripts/gen_routes.py $(ARTEX_SRC)

install: build
	install -m 0755 $(BINARY) $(HOME)/.local/bin/artex-mcp

clean:
	rm -rf bin
