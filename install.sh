#!/usr/bin/env sh
# envkeep installer — downloads the latest release binary for your OS/arch.
#
#   curl -sSfL https://raw.githubusercontent.com/carvalhosauro/envkeep/main/install.sh | sh
#
# Override the install dir with ENVKEEP_INSTALL_DIR (default: ~/.local/bin).
# Needs: curl, tar. No Go toolchain required.
set -eu

REPO="carvalhosauro/envkeep"
BIN="envkeep"
INSTALL_DIR="${ENVKEEP_INSTALL_DIR:-$HOME/.local/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "envkeep: unsupported arch: $arch" >&2; exit 1 ;;
esac
case "$os" in
  linux | darwin) ;;
  *) echo "envkeep: unsupported os: $os" >&2; exit 1 ;;
esac

tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
  | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
if [ -z "$tag" ]; then
  echo "envkeep: could not determine the latest release (is one published?)" >&2
  exit 1
fi

archive="${BIN}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$tag/$archive"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "envkeep: downloading $tag ($os/$arch)"
curl -fSL "$url" -o "$tmp/$archive"
tar -C "$tmp" -xzf "$tmp/$archive"

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/$BIN" "$INSTALL_DIR/$BIN"

echo "envkeep: installed to $INSTALL_DIR/$BIN"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "envkeep: note — $INSTALL_DIR is not on your PATH" >&2 ;;
esac
"$INSTALL_DIR/$BIN" version || true
