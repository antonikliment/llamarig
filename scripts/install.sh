#!/usr/bin/env sh
# Installs llamarig from GitHub releases.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/antonikliment/llamarig/main/scripts/install.sh | sh
#   curl -fsSL .../install.sh | sh -s -- v0.1.0-alpha.2   # install a specific version
#
# Env vars:
#   INSTALL_DIR   where to place the binary (default: /usr/local/bin, falls back to ~/.local/bin)
set -eu

repo="antonikliment/llamarig"
version=${1:-}

os=$(uname -s)
arch=$(uname -m)

case "$os" in
Linux) os=linux ;;
Darwin) os=darwin ;;
*)
	echo "unsupported OS: $os" >&2
	exit 1
	;;
esac

case "$arch" in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*)
	echo "unsupported architecture: $arch" >&2
	exit 1
	;;
esac

if [ -z "$version" ]; then
	version=$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
	if [ -z "$version" ]; then
		echo "could not determine latest release version" >&2
		exit 1
	fi
fi

name="llamarig_${version}_${os}_${arch}"
base_url="https://github.com/$repo/releases/download/$version"

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT

echo "downloading $name.tar.gz ($version)..." >&2
curl -fsSL -o "$workdir/$name.tar.gz" "$base_url/$name.tar.gz"
curl -fsSL -o "$workdir/SHA256SUMS" "$base_url/SHA256SUMS"

(cd "$workdir" && sha256sum --ignore-missing --check SHA256SUMS)

tar -C "$workdir" -xzf "$workdir/$name.tar.gz"

install_dir=${INSTALL_DIR:-/usr/local/bin}
if [ ! -w "$install_dir" ] 2>/dev/null; then
	install_dir="${INSTALL_DIR:-$HOME/.local/bin}"
	mkdir -p "$install_dir"
fi

install -m 755 "$workdir/$name/llamarig" "$install_dir/llamarig"
echo "installed $("$install_dir/llamarig" version) to $install_dir/llamarig" >&2

case ":$PATH:" in
*":$install_dir:"*) ;;
*) echo "note: $install_dir is not on your PATH" >&2 ;;
esac
