# Plan: curl installer for gojekyll

## Goal

Give non-Go users a one-line install:

```bash
curl -fsSL https://raw.githubusercontent.com/reidransom/gojekyll/main/install.sh | sh
```

…that downloads the correct prebuilt archive from GitHub Releases, installs the
`gojekyll` binary, **and** preserves the bundled `dart-sass/` directory it depends on.

## What already exists (don't rebuild)

- GoReleaser (`.goreleaser.yaml`) + `.github/workflows/goreleaser.yml` already publish
  per-tag GitHub Releases with cross-platform tarballs and a `checksums.txt`.
- Asset name template (uname-friendly):
  `gojekyll_{title .Os}_{x86_64|arm64|armvN|riscv64}{amd64 variant v1/v2/v3}{-musl}.tar.gz`
  - macOS: `gojekyll_Darwin_arm64.tar.gz`, `gojekyll_Darwin_x86_64v1.tar.gz`
  - Linux glibc: `gojekyll_Linux_x86_64v1.tar.gz`, `gojekyll_Linux_arm64.tar.gz`
  - Linux musl: `gojekyll_Linux_x86_64v1-musl.tar.gz`, `gojekyll_Linux_arm64-musl.tar.gz`
  - Windows: `.zip` (out of scope for a `sh` installer)
- `checksums.txt` is published per release for verification.

## Critical constraint: dart-sass is bundled, not standalone

`renderers/sass.go` (`sassExecutable`, ~line 98) resolves dart-sass as:

1. `sass` on `PATH` (preferred), else
2. `<dir of gojekyll executable>/dart-sass/sass`.

Each release archive ships `gojekyll` **and** a sibling `dart-sass/` directory.
**Implication:** the installer must place `dart-sass/` next to the installed
`gojekyll` binary. A naive `mv gojekyll /usr/local/bin/` (discarding the rest)
breaks SASS rendering for users without `sass` on PATH.

Two viable layouts:
- **A — bin + sibling dir (chosen):** install `gojekyll` to `$BIN_DIR/gojekyll`
  and `dart-sass/` to `$BIN_DIR/dart-sass/`. Matches the lookup exactly; one dir on PATH.
- B — self-contained dir + symlink: install whole archive to
  `~/.local/share/gojekyll/` and symlink `gojekyll` into a PATH dir. Cleaner
  isolation, but `os.Executable()` resolves symlinks so the `dart-sass/` must sit
  next to the *real* binary (it does), so this also works. More moving parts.

Going with **A** for simplicity.

## amd64 micro-architecture level (v1/v2/v3)

GoReleaser splits amd64 into `v1`/`v2`/`v3` (SSE/AVX feature levels). `uname -m`
only reports `x86_64`. Safe default: **download `v1`** (baseline, runs everywhere).
Optionally detect `v3` via `/proc/cpuinfo` flags on Linux, but baseline `v1` is the
correct, low-risk default — document that advanced users can `go install` for an
optimized build. arm64/armvN/riscv64 map directly from `uname -m`.

## install.sh design

Location: repo root `install.sh` (served raw from `main`). Plain POSIX `sh`.

Steps:
1. `set -eu`; define `REPO="reidransom/gojekyll"`, `BINARY="gojekyll"`.
2. Detect OS: `uname -s` → `Linux`→`Linux`, `Darwin`→`Darwin`; bail on anything else
   (point Windows users at releases page / `go install`).
3. Detect arch from `uname -m`:
   - `x86_64|amd64` → `x86_64v1`
   - `arm64|aarch64` → `arm64`
   - `armv7l` → `armv7`, `armv6l` → `armv6` (Linux only)
   - `riscv64` → `riscv64` (Linux only)
   - else → error with a clear message.
4. Detect libc on Linux: if `ldd --version 2>&1 | grep -qi musl` (or
   `[ -f /etc/alpine-release ]`), append `-musl`. macOS: never musl.
5. Resolve version: default `latest` via GitHub API
   `https://api.github.com/repos/$REPO/releases/latest` → parse `tag_name`
   (grep/sed, no jq dependency). Allow override via `VERSION=v1.0.1` env var.
6. Build asset name: `${BINARY}_${OS}_${ARCH}.tar.gz` and URL
   `https://github.com/$REPO/releases/download/$VERSION/$ASSET`.
7. `TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT`.
8. Download archive with `curl -fsSL` (fallback to `wget` if no curl).
9. **Verify checksum:** download `checksums.txt`, grep the asset line, compare with
   `sha256sum`/`shasum -a 256`. Warn-and-continue if no sha tool present; never
   silently skip.
10. `tar -C "$TMP" -xzf` the archive.
11. Choose `BIN_DIR`:
    - `INSTALL_DIR` env override wins.
    - else if `$HOME/.local/bin` exists or can be made → use it (no sudo).
    - else `/usr/local/bin` (use `sudo` only if not writable and a tty is available).
12. Install: `install -m 0755 gojekyll "$BIN_DIR/gojekyll"`; copy `dart-sass/` →
    `"$BIN_DIR/dart-sass/"` (rm old first to avoid stale files). Preserve the
    `sass` executable bit.
13. PATH hint: if `$BIN_DIR` not on `$PATH`, print the exact `export PATH=...` line
    for the user's shell.
14. Verify: run `"$BIN_DIR/gojekyll" version` and echo the result.

### Robustness / polish
- `curl | sh` safety: document the inspect-first alternative
  (`curl -fsSL …/install.sh -o install.sh; less install.sh; sh install.sh`).
- Idempotent: re-running upgrades in place.
- Clear, prefixed log lines (`gojekyll-install:`), all diagnostics to stderr.
- Exit non-zero with actionable messages on every failure path.

## README changes

Add an **Install** section:

```bash
# Quick install (macOS / Linux)
curl -fsSL https://raw.githubusercontent.com/reidransom/gojekyll/main/install.sh | sh

# Pin a version
curl -fsSL https://raw.githubusercontent.com/reidransom/gojekyll/main/install.sh | VERSION=v1.0.1 sh

# Go developers
go install github.com/reidransom/gojekyll@latest
```

Note: the curl installer bundles dart-sass; `go install` does not, so `go install`
users need `sass` on their PATH for SCSS support.

## Testing

1. `shellcheck install.sh` (add to CI optionally).
2. Dry-run locally on this Mac (arm64): pipe to `sh` with
   `INSTALL_DIR=/tmp/gjtest`, confirm `gojekyll` + `dart-sass/` land together and
   `version` works.
3. Confirm a SCSS render works using the bundled dart-sass (PATH `sass` removed):
   build a small site that imports SCSS from `/tmp/gjtest`.
4. Verify the GitHub API `latest` parse against the real release.
5. Test `VERSION=` pinning and an unsupported-arch error path.

## Out of scope (future)

- Windows `.zip` PowerShell installer (`install.ps1`).
- Homebrew tap (GoReleaser can generate `brews:`), `.deb`/`.rpm`.
- amd64 `v2`/`v3` auto-detection.
- A vanity domain (`get.…dev`) — using the raw GitHub URL avoids hosting.

## Steps

1. Write `install.sh` at repo root per the design above.
2. `shellcheck` and local dry-run install to `/tmp/gjtest` (incl. dart-sass + SCSS check).
3. Add the Install section to `README.md`.
4. (Optional) Add a `shellcheck install.sh` step to a CI workflow.
5. Commit; the script is live from `main` immediately (no release needed since it
   reads `latest` from the API).
