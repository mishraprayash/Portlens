BINARY := portlens
GO ?= go

# CGO_ENABLED=0 is required: PortLens is intentionally free of cgo so it can be
# cross-compiled trivially and produces static binaries. It also sidesteps a
# linker incompatibility between older Go toolchains and recent macOS releases.
export CGO_ENABLED := 0

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/portlens/portlens/internal/version.Version=$(VERSION)

.PHONY: build build-release install test vet fmt lint check clean cross bench profile race

build:
	$(GO) build -ldflags '$(LDFLAGS)' -o bin/$(BINARY) .

# A small, reproducible release build: no build path metadata, stripped DWARF,
# and the version stamp. Produces bin/portlens-release.
build-release:
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/$(BINARY)-release .

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

# bench runs the performance regression suite (see docs/performance.md).
bench:
	$(GO) test -run '^$$' -bench . -benchmem -count=1 ./internal/...

# profile writes CPU/allocation profiles for the fast and deep inspection paths.
profile:
	$(GO) test -run '^$$' -bench 'BenchmarkInspectPort(Fast)?$$' -benchtime 20x \
		-cpuprofile /tmp/portlens-cpu.out -memprofile /tmp/portlens-mem.out \
		./internal/inspector/
	@echo "CPU:     go tool pprof /tmp/portlens-cpu.out"
	@echo "Memory:  go tool pprof /tmp/portlens-mem.out"

# race runs the race detector. NOTE: on macOS this is blocked by a pre-existing
# gopsutil/purego incompatibility with the Go race runtime (see
# docs/performance.md); CI does not run -race.
race:
	$(GO) test -race -count=1 ./...

clean:
	rm -rf bin

# Cross-compile checks (no cgo needed).
cross:
	GOOS=darwin GOARCH=arm64 $(GO) build -o /dev/null .
	GOOS=darwin GOARCH=amd64 $(GO) build -o /dev/null .
	GOOS=linux  GOARCH=amd64 $(GO) build -o /dev/null .
	GOOS=linux  GOARCH=arm64 $(GO) build -o /dev/null .
