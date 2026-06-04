#!/bin/sh
# jigyll installer for macOS and Linux.
#
#   curl -fsSL https://raw.githubusercontent.com/reidransom/jigyll/main/install.sh | sh
#
# Installs the jigyll binary from GitHub Releases and, unless `sass` is already
# on PATH, a matching dart-sass (jigyll resolves dart-sass from PATH).
#
# Env overrides:
#   VERSION=v1.0.1      pin a jigyll release (default: latest)
#   INSTALL_DIR=/path   install location (default: ~/.local/bin, else /usr/local/bin)
#   DART_SASS_VERSION   dart-sass version to fetch (default: pinned below)
set -eu

REPO="reidransom/jigyll"
BINARY="jigyll"
DART_SASS_VERSION="${DART_SASS_VERSION:-1.98.0}"

say() { printf '%s: %s\n' "$BINARY-install" "$1" >&2; }
die() { say "error: $1"; exit 1; }

# --- download helper (curl or wget) ------------------------------------------
if command -v curl >/dev/null 2>&1; then
  dl() { curl -fsSL "$1" -o "$2"; }
  dl_stdout() { curl -fsSL "$1"; }
elif command -v wget >/dev/null 2>&1; then
  dl() { wget -qO "$2" "$1"; }
  dl_stdout() { wget -qO- "$1"; }
else
  die "need curl or wget"
fi

# --- detect OS ---------------------------------------------------------------
os_uname=$(uname -s)
case "$os_uname" in
  Linux)  OS=Linux;  DART_OS=linux ;;
  Darwin) OS=Darwin; DART_OS=macos ;;
  *) die "unsupported OS '$os_uname'. Windows users: see releases, or 'go install github.com/$REPO@latest' with sass on PATH." ;;
esac

# --- detect arch -------------------------------------------------------------
# GJ_ARCH feeds the jigyll asset name; DART_ARCH feeds the dart-sass asset.
arch_uname=$(uname -m)
case "$arch_uname" in
  x86_64|amd64)  GJ_ARCH=x86_64v1; DART_ARCH=x64 ;;
  arm64|aarch64) GJ_ARCH=arm64;    DART_ARCH=arm64 ;;
  armv7l)        GJ_ARCH=armv7;    DART_ARCH=arm ;;
  armv6l)        GJ_ARCH=armv6;    DART_ARCH=arm ;;
  riscv64)       GJ_ARCH=riscv64;  DART_ARCH=riscv64 ;;
  *) die "unsupported architecture '$arch_uname'" ;;
esac

# dart-sass ships a musl variant for x64/arm64 on Alpine/musl systems.
DART_LIBC=""
if [ "$OS" = "Linux" ]; then
  if [ -f /etc/alpine-release ] || (ldd --version 2>&1 | grep -qi musl); then
    case "$DART_ARCH" in
      x64|arm64) DART_LIBC="-musl" ;;
    esac
  fi
fi

# --- resolve jigyll version ------------------------------------------------
VERSION="${VERSION:-}"
if [ -z "$VERSION" ]; then
  say "resolving latest release"
  VERSION=$(dl_stdout "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
  [ -n "$VERSION" ] || die "could not determine latest version (set VERSION=vX.Y.Z)"
fi

# --- choose install dir ------------------------------------------------------
SUDO=""
if [ -n "${INSTALL_DIR:-}" ]; then
  BIN_DIR="$INSTALL_DIR"
elif [ -d "$HOME/.local/bin" ] || mkdir -p "$HOME/.local/bin" 2>/dev/null; then
  BIN_DIR="$HOME/.local/bin"
else
  BIN_DIR="/usr/local/bin"
fi
mkdir -p "$BIN_DIR" 2>/dev/null || true
if [ ! -w "$BIN_DIR" ]; then
  if command -v sudo >/dev/null 2>&1 && [ -t 0 ]; then
    SUDO="sudo"
    say "$BIN_DIR is not writable; using sudo"
  else
    die "$BIN_DIR is not writable (set INSTALL_DIR to a writable path)"
  fi
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

# --- install jigyll --------------------------------------------------------
GJ_ASSET="${BINARY}_${OS}_${GJ_ARCH}.tar.gz"
GJ_URL="https://github.com/$REPO/releases/download/$VERSION/$GJ_ASSET"
say "downloading $GJ_ASSET ($VERSION)"
dl "$GJ_URL" "$TMP/$GJ_ASSET" || die "download failed: $GJ_URL"

# verify checksum against the release's checksums.txt
if dl "https://github.com/$REPO/releases/download/$VERSION/checksums.txt" "$TMP/checksums.txt" 2>/dev/null; then
  expected=$(grep " $GJ_ASSET\$" "$TMP/checksums.txt" | awk '{print $1}')
  if [ -n "$expected" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      actual=$(sha256sum "$TMP/$GJ_ASSET" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
      actual=$(shasum -a 256 "$TMP/$GJ_ASSET" | awk '{print $1}')
    else
      actual=""; say "no sha256 tool found; skipping checksum verification"
    fi
    [ -z "$actual" ] || [ "$actual" = "$expected" ] || die "checksum mismatch for $GJ_ASSET"
  else
    say "warning: $GJ_ASSET not listed in checksums.txt; skipping verification"
  fi
else
  say "warning: could not fetch checksums.txt; skipping verification"
fi

tar -C "$TMP" -xzf "$TMP/$GJ_ASSET" || die "failed to extract $GJ_ASSET"
[ -f "$TMP/$BINARY" ] || die "archive did not contain '$BINARY'"
$SUDO install -m 0755 "$TMP/$BINARY" "$BIN_DIR/$BINARY"
say "installed $BIN_DIR/$BINARY"

# --- install dart-sass (unless sass already on PATH) -------------------------
if command -v sass >/dev/null 2>&1; then
  say "found existing sass on PATH ($(command -v sass)); skipping dart-sass"
else
  DART_PLATFORM="${DART_OS}-${DART_ARCH}${DART_LIBC}"
  DART_ASSET="dart-sass-${DART_SASS_VERSION}-${DART_PLATFORM}.tar.gz"
  DART_URL="https://github.com/sass/dart-sass/releases/download/${DART_SASS_VERSION}/${DART_ASSET}"
  say "downloading $DART_ASSET"
  if dl "$DART_URL" "$TMP/$DART_ASSET" 2>/dev/null && tar -C "$TMP" -xzf "$TMP/$DART_ASSET" 2>/dev/null; then
    $SUDO rm -rf "$BIN_DIR/dart-sass"
    $SUDO cp -R "$TMP/dart-sass" "$BIN_DIR/dart-sass"
    $SUDO chmod 0755 "$BIN_DIR/dart-sass/sass"
    $SUDO ln -sf "$BIN_DIR/dart-sass/sass" "$BIN_DIR/sass"
    say "installed dart-sass + sass symlink in $BIN_DIR"
  else
    say "warning: no dart-sass build for $DART_PLATFORM; install 'sass' on PATH for SCSS support"
  fi
fi

# --- PATH hint ---------------------------------------------------------------
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) say "note: $BIN_DIR is not on your PATH. Add it:"
     printf '    export PATH="%s:$PATH"\n' "$BIN_DIR" >&2 ;;
esac

# --- verify ------------------------------------------------------------------
say "verifying"
"$BIN_DIR/$BINARY" version >&2 || say "warning: '$BINARY version' did not run cleanly"
if [ -x "$BIN_DIR/sass" ]; then
  "$BIN_DIR/sass" --version >&2 || true
fi
say "done"
