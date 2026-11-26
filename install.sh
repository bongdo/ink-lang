#!/bin/sh
set -eu

repo="bongdo/ink-lang"
dest="${INK_INSTALL_DIR:-$HOME/.local/bin}"

case "$(uname -s)" in
Linux) os=linux ;;
Darwin) os=darwin ;;
*)
	echo "install.sh: unsupported OS: $(uname -s)" >&2
	exit 1
	;;
esac

case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*)
	echo "install.sh: unsupported architecture: $(uname -m)" >&2
	exit 1
	;;
esac

if [ -n "${INK_BASE_URL:-}" ]; then
	base="$INK_BASE_URL"
elif [ -n "${INK_VERSION:-}" ]; then
	version="${INK_VERSION#v}"
	base="https://github.com/$repo/releases/download/v$version"
else
	base="https://github.com/$repo/releases/latest/download"
fi

archive="ink_${os}_${arch}.tar.gz"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fetch() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1" -o "$2"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$2" "$1"
	else
		echo "install.sh: curl or wget is required" >&2
		exit 1
	fi
}

fetch "$base/$archive" "$tmp/$archive"
fetch "$base/checksums.txt" "$tmp/checksums.txt"

line="$(grep -E "[[:space:]]${archive}$" "$tmp/checksums.txt" || true)"
if [ -z "$line" ]; then
	echo "install.sh: no checksum for $archive" >&2
	exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
	printf '%s\n' "$line" | (cd "$tmp" && sha256sum -c - >/dev/null)
elif command -v shasum >/dev/null 2>&1; then
	printf '%s\n' "$line" | (cd "$tmp" && shasum -a 256 -c - >/dev/null)
else
	echo "install.sh: sha256sum or shasum is required" >&2
	exit 1
fi

tar -xzf "$tmp/$archive" -C "$tmp"
bin="$tmp/ink"
if [ ! -f "$bin" ]; then
	bin="$(find "$tmp" -type f -name ink | head -n 1)"
fi
if [ -z "$bin" ] || [ ! -f "$bin" ]; then
	echo "install.sh: archive does not contain ink" >&2
	exit 1
fi

mkdir -p "$dest"
install -m 755 "$bin" "$dest/ink"
echo "installed $dest/ink"
"$dest/ink" version
case ":$PATH:" in
*":$dest:"*) ;;
*) echo "add $dest to PATH" ;;
esac
