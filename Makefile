BINARY := portlens
GO ?= go

# CGO_ENABLED=0 is required: PortLens is intentionally free of cgo so it can be
# cross-compiled trivially and produces static binaries. It also sidesteps a
# linker incompatibility between older Go toolchains and recent macOS releases.
export CGO_ENABLED := 0

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/portlens/portlens/internal/version.Version=$(VERSION)

.PHONY: build install test vet fmt lint clean

build:
	$(GO) build -ldflags '$(LDFLAGS)' -o bin/$(BINARY) .

install:
	$(GO) install -ldflags '$(LDFLAGS)' .

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

clean:
	rm -rf bin

# Cross-compile checks (no cgo needed).
cross:
	GOOS=darwin GOARCH=arm64 $(GO) build -o /dev/null .
	GOOS=darwin GOARCH=amd64 $(GO) build -o /dev/null .
	GOOS=linux  GOARCH=amd64 $(GO) build -o /dev/null .
	GOOS=linux  GOARCH=arm64 $(GO) build -o /dev/null .
