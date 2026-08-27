# Installation

This guide covers installing PortLens on macOS and Linux, building from source,
cross-compiling for another platform, verifying the install, and uninstalling.

---

## Prerequisites

- **Go 1.23+** — required only if you build from source (the current install
  path, since no prebuilt packages are published yet).
- **macOS** or **Linux**. (Windows is planned; the codebase is structured for
  it.)

---

## Option 1 — Build from source (recommended)

### 1. Install Go

**macOS**

```bash
brew install go
```

**Linux** (Ubuntu/Debian)

```bash
sudo apt-get update && sudo apt-get install -y golang-go
```

**Linux** (Fedora)

```bash
sudo dnf install golang
```

**Linux** (Arch)

```bash
sudo pacman -S go
```

Or install the official tarball: <https://go.dev/dl/>.

### 2. Clone and install

```bash
git clone https://github.com/portlens/portlens.git
cd portlens
make install
```

`make install` builds and installs `portlens` to your `$GOBIN` (default
`~/go/bin`). It automatically sets `CGO_ENABLED=0`, which is required — see
[Why cgo-free?](#why-cgo-free) below.

> **Prefer the Makefile over a raw `go install`.** A plain `go install .`
> enables cgo by default, which produces a binary that crashes on recent macOS
> (`missing LC_UUID load command`) when built with Go 1.23.x. Use one of these
> instead:
>
> ```bash
> make install                 # recommended
> CGO_ENABLED=0 go install .   # explicit
> go env -w CGO_ENABLED=0 && go install .   # set permanently, then install
> ```

### 3. Make sure `portlens` is on your `PATH`

`go install` places binaries in `$GOBIN`, which defaults to `$(go env
GOPATH)/bin` (`~/go/bin`). Add it to your shell if it isn't already:

```bash
# zsh
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc
# bash
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc
```

Then open a new terminal (or `source` the file) and verify:

```bash
$ portlens --version
portlens 0.1.0
```

---

## Option 2 — Build a single binary

If you don't want Go on your target machine, build on any machine with Go and
copy the static binary over:

```bash
cd portlens
make build                  # produces ./bin/portlens
sudo cp bin/portlens /usr/local/bin/portlens
```

PortLens is a single, static, dependency-free binary — no runtime libraries, no
cgo, no shared objects.

---

## Option 3 — Cross-compile for another platform

Because PortLens is cgo-free, you can build for any target from any host:

```bash
# On any machine: build a Linux amd64 binary
GOOS=linux  GOARCH=amd64 CGO_ENABLED=0 go build -o portlens-linux-amd64 .

# Build a macOS Apple Silicon binary
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o portlens-darwin-arm64 .
```

| Target                 | `GOOS`  | `GOARCH` |
|------------------------|---------|----------|
| macOS (Apple Silicon)  | darwin  | arm64    |
| macOS (Intel)          | darwin  | amd64    |
| Linux (x86-64)         | linux   | amd64    |
| Linux (ARM64)          | linux   | arm64    |

Copy the resulting binary to the target and place it on `PATH` (e.g.
`/usr/local/bin/portlens`).

---

## Verify the installation

```bash
$ portlens --version
portlens 0.1.0

$ portlens --help        # shows the command reference

$ portlens               # lists listening ports
$ portlens 3000          # inspects a specific port
```

---

## Uninstall

```bash
# Remove the binary
rm -f "$(command -v portlens)"

# Remove local history (optional)
rm -rf "$HOME/Library/Application Support/portlens"   # macOS
rm -rf "$HOME/.local/share/portlens"                  # Linux
```

---

## Package managers & releases

Prebuilt releases and package managers are on the roadmap but not yet published:

- **Homebrew tap** (macOS) — planned.
- **`go install github.com/portlens/portlens@latest`** — will work once the
  module is published to the Go proxy (requires the same
  `CGO_ENABLED=0` note above).
- **Distro packages / apt / AUR** — planned.

Until then, build from source or cross-compile a binary (Options 1–3).

---

## Why cgo-free?

PortLens is built with `CGO_ENABLED=0`:

1. **Static, portable binaries** — no libc dependency, trivially cross-compiled.
2. **Fewer moving parts** — no C compiler or SDK required to build.
3. **Avoids a known toolchain bug** — Go 1.23.x binaries that link cgo (via the
   `net` package's resolver) crash on recent macOS with
   `dyld: missing LC_UUID load command`. The cgo-free build sidesteps this
   entirely. (Go 1.24+ fixes the underlying issue.)

The `Makefile` sets `CGO_ENABLED=0` automatically for `build` and `install`.

---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `command not found: portlens` | Add `$(go env GOPATH)/bin` to your `PATH` (see above). |
| `dyld: missing LC_UUID load command` | You built with cgo on an old Go + recent macOS. Rebuild with `CGO_ENABLED=0` or upgrade Go to 1.24+. |
| `lsof: command not found` (Linux) | Install lsof (`sudo apt-get install lsof`), or rely on the native `/proc` provider on standard distros. |
| "Owner could not be determined" | The process may require elevated privileges to inspect; PortLens won't escalate. |
| Clipboard actions fail | Install a clipboard tool: macOS has `pbcopy` built-in; on Linux install `wl-copy` (Wayland) or `xclip`/`xsel` (X11). |
| `permission denied` when killing | The process is owned by another user. PortLens will not attempt `sudo`. |
