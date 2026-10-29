BINARY := portlens
GO ?= go

# CGO_ENABLED=0 is required: PortLens is intentionally free of cgo so it can be
# cross-compiled trivially and produces static binaries. It also sidesteps a
# linker incompatibility between older Go toolchains and recent macOS releases.
export CGO_ENABLED := 0

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/mishraprayash/Portlens/internal/version.Version=$(VERSION)

.PHONY: build build-release install test vet fmt lint check cover clean cross bench profile race

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

# cover runs the full suite with a whole-module coverage profile and prints the
# aggregate statement percentage (see docs/testing.md).
cover:
	$(GO) test -count=1 -coverpkg=./... -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -n 1

# check is the full local gate: formatting, vet, build, tests, and the
# cross-compile matrix. CI runs the same checks (see .github/workflows/ci.yml).
check: lint build test cross

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

# race runs the race detector. NOTE: only on Linux. The race runtime requires
# cgo, and on macOS Go 1.23.x produces test binaries missing the LC_UUID load
# command that dyld refuses to run; CI runs -race on ubuntu only.
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
