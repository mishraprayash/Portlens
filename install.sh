#!/bin/sh
# Install the latest PortLens release binary for macOS or Linux.
#   curl -fsSL https://raw.githubusercontent.com/mishraprayash/Portlens/main/install.sh | sh
# Environment: PORTLENS_VERSION (default: latest, e.g. v0.3.0), INSTALL_DIR (default: /usr/local/bin).
set -eu

repo="mishraprayash/Portlens"
install_dir="${INSTALL_DIR:-/usr/local/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux|darwin) ;;
  *) echo "portlens: unsupported OS: $os" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "portlens: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

tag="${PORTLENS_VERSION:-}"
if [ -z "$tag" ]; then
  tag=$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" |
    sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
fi
[ -n "$tag" ] || { echo "portlens: could not determine latest release" >&2; exit 1; }
version="${tag#v}"

archive="portlens_${version}_${os}_${arch}.tar.gz"
base="https://github.com/$repo/releases/download/$tag"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $archive ($tag)..."
curl -fsSL -o "$tmp/$archive" "$base/$archive"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt"

expected=$(grep " $archive\$" "$tmp/checksums.txt" | awk '{print $1}')
[ -n "$expected" ] || { echo "portlens: no checksum for $archive" >&2; exit 1; }
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$archive" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')
fi
[ "$expected" = "$actual" ] || { echo "portlens: checksum mismatch" >&2; exit 1; }

tar -xzf "$tmp/$archive" -C "$tmp" portlens
if [ -w "$install_dir" ]; then
  install -m 0755 "$tmp/portlens" "$install_dir/portlens"
else
  echo "Installing to $install_dir (sudo required)..."
  sudo install -m 0755 "$tmp/portlens" "$install_dir/portlens"
fi
echo "Installed: $("$install_dir/portlens" --version)"
