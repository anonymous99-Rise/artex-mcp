BINARY  := bin/artex-mcp
VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: all build test vet fmt tidy routes release clean install

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

# Cross-compile release binaries and checksums into dist/.
# Usage: make release VERSION=v1.0.0  (defaults to git describe)
release:
	rm -rf dist
	mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" \
			-o dist/artex-mcp_$(VERSION)_$${os}_$${arch} ./cmd/artex-mcp || exit 1; \
	done
	cd dist && sha256sum * > checksums.txt && cat checksums.txt

install: build
	install -m 0755 $(BINARY) $(HOME)/.local/bin/artex-mcp

clean:
	rm -rf bin
