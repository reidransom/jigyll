#!/usr/bin/env bash
set -euo pipefail

DART_SASS_VERSION="${DART_SASS_VERSION:-1.98.0}"
BASE_URL="https://github.com/sass/dart-sass/releases/download/${DART_SASS_VERSION}"
DEST_DIR=".dart-sass"

mkdir -p "$DEST_DIR"

download_sass() {
    local go_os=$1    # linux, darwin, windows (goreleaser naming)
    local go_arch=$2  # amd64, arm64, arm, riscv64 (goreleaser naming)
    local dart_platform=$3  # e.g. linux-x64, macos-arm64, windows-x64
    local ext=${4:-tar.gz}

    local dest="${DEST_DIR}/${go_os}-${go_arch}"

    if [ -d "$dest" ]; then
        echo "Skipping ${go_os}-${go_arch} (already exists)"
        return 0
    fi

    local url="${BASE_URL}/dart-sass-${DART_SASS_VERSION}-${dart_platform}.${ext}"
    echo "Downloading dart-sass ${DART_SASS_VERSION} for ${go_os}/${go_arch}..."

    local tmpdir
    tmpdir=$(mktemp -d)

    if [ "$ext" = "zip" ]; then
        curl -fsSL "$url" -o "${tmpdir}/dart-sass.zip"
        unzip -q "${tmpdir}/dart-sass.zip" -d "$tmpdir"
    else
        curl -fsSL "$url" | tar -xz -C "$tmpdir"
    fi

    mkdir -p "$dest"
    mv "${tmpdir}/dart-sass/"* "$dest/"
    rm -rf "$tmpdir"
    echo "  -> ${dest}/"
}

# Linux
download_sass linux  amd64   linux-x64
download_sass linux  arm64   linux-arm64
download_sass linux  arm     linux-arm
download_sass linux  riscv64 linux-riscv64

# macOS
download_sass darwin amd64   macos-x64
download_sass darwin arm64   macos-arm64

# Windows
download_sass windows amd64  windows-x64  zip
download_sass windows arm64  windows-arm64 zip

echo "Done. dart-sass ${DART_SASS_VERSION} downloaded for all platforms."
