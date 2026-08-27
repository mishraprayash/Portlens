BINARY := portlens
GO ?= go

# CGO_ENABLED=0 is required: PortLens is intentionally free of cgo so it can be
# cross-compiled trivially and produces static binaries. It also sidesteps a
# linker incompatibility between older Go toolchains and recent macOS releases.
export CGO_ENABLED := 0

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/portlens/portlens/internal/version.Version=$(VERSION)

.PHONY: build install test vet fmt lint check clean cross

build:
	$(GO) build -ldflags '$(LDFLAGS)' -o bin/$(BINARY) .

install:
	$(GO) install -ldflags '$(LDFLAGS)' .

# -count=1 disables the test cache so results are always fresh.
test:
	$(GO) test -count=1 ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

# lint fails on any non-gofmt-formatted file, then runs go vet.
lint:
	@files=$$(gofmt -l .); \
	if [ -n "$$files" ]; then \
		echo "Files not gofmt-formatted:"; \
		echo "$$files"; \
		echo "Run 'make fmt' to fix."; \
		exit 1; \
	fi
	$(GO) vet ./...

# check is the full local gate; CI runs the same set of checks.
check: lint test

clean:
	rm -rf bin

# Cross-compile checks (no cgo needed).
cross:
	GOOS=darwin GOARCH=arm64 $(GO) build -o /dev/null .
	GOOS=darwin GOARCH=amd64 $(GO) build -o /dev/null .
	GOOS=linux  GOARCH=amd64 $(GO) build -o /dev/null .
	GOOS=linux  GOARCH=arm64 $(GO) build -o /dev/null .
