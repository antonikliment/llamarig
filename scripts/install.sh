#!/usr/bin/env sh
# Installs llamarig, either as a native binary from GitHub releases or as a
# Docker container.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/antonikliment/llamarig/main/scripts/install.sh | sh
#   curl -fsSL .../install.sh | sh -s -- v0.1.0-alpha.2   # pin a release (binary)
#   curl -fsSL .../install.sh | sh -s -- --docker         # run via Docker instead
#
# When neither --binary nor --docker is given and the script is run
# interactively, it asks. Piped (non-interactive) runs default to the binary.
#
# Env vars:
#   LLAMARIG_MODE   binary | docker (skips the prompt)
#   INSTALL_DIR     binary mode: where to place the binary
#                   (default: /usr/local/bin, falls back to ~/.local/bin)
#   LLAMARIG_DIR    docker mode: checkout dir (default: ~/.llamarig-docker)
set -eu

repo="antonikliment/llamarig"
repo_url="https://github.com/$repo.git"
version=""
mode="${LLAMARIG_MODE:-}"

for arg in "$@"; do
	case "$arg" in
	--docker) mode=docker ;;
	--binary) mode=binary ;;
	-h | --help)
		sed -n '2,20p' "$0" 2>/dev/null || echo "see script header for usage" >&2
		exit 0
		;;
	v*) version=$arg ;;
	*)
		echo "unknown argument: $arg" >&2
		exit 1
		;;
	esac
done

# Ask when the mode is unset and we have a terminal to ask on.
if [ -z "$mode" ]; then
	if [ -t 0 ]; then
		printf 'Install method: [1] native binary (default)  [2] Docker container: ' >&2
		read -r choice || choice=1
		case "$choice" in
		2 | d | docker) mode=docker ;;
		*) mode=binary ;;
		esac
	else
		mode=binary
	fi
fi

install_docker() {
	if ! command -v docker >/dev/null 2>&1; then
		echo "docker is not installed or not on PATH" >&2
		exit 1
	fi
	if ! command -v git >/dev/null 2>&1; then
		echo "git is required to fetch the Docker build context" >&2
		exit 1
	fi
	dir=${LLAMARIG_DIR:-$HOME/.llamarig-docker}
	if [ -d "$dir/.git" ]; then
		echo "updating existing checkout in $dir..." >&2
		git -C "$dir" pull --ff-only
	else
		echo "cloning $repo into $dir..." >&2
		git clone --depth 1 "$repo_url" "$dir"
	fi
	echo "building and starting the container..." >&2
	(cd "$dir" && docker compose up -d --build)
	echo "llamarig is running at http://127.0.0.1:7000/" >&2
	echo "manage it with: cd $dir && docker compose {logs,down,up}" >&2
}

install_binary() {
	os=$(uname -s)
	arch=$(uname -m)

	case "$os" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*)
		echo "unsupported OS: $os (try --docker)" >&2
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
		# Use the releases list, not /releases/latest: the latter excludes
		# prereleases and 404s while only prereleases exist. The list is newest
		# first, so the first tag_name is the most recent release.
		version=$(curl -fsSL "https://api.github.com/repos/$repo/releases?per_page=1" | grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
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
}

case "$mode" in
docker) install_docker ;;
binary) install_binary ;;
*)
	echo "unknown install mode: $mode" >&2
	exit 1
	;;
esac
