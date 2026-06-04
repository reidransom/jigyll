# Plan: curl installer for gojekyll

## Goal

Give non-Go users a one-line install:

```bash
curl -fsSL https://raw.githubusercontent.com/reidransom/gojekyll/main/install.sh | sh
```

…that downloads the correct prebuilt `gojekyll` archive from GitHub Releases **and**
installs a working dart-sass (`sass`) onto PATH, since gojekyll resolves dart-sass
via PATH only (see Sass decision below).

## Sass handling: adopt upstream PATH-only (decided)

We are **fully adopting upstream `osteele/gojekyll`'s Sass approach** and making the
installer responsible for dart-sass. Rationale: zero fork divergence in the Sass
code, a slimmer release pipeline, and we pick up upstream's logger migration,
`sasserrors.Enhance`, and stale-CSS temp-dir cleanup (which our fork currently
lacks).

Concretely, upstream resolves the dart-sass binary as a **global singleton that
only looks on PATH**:

```go
// renderers/renderers.go (upstream)
func (p *Manager) getSassTranspiler() (*sass.Transpiler, error) {
    globalSassTranspilerOnce.Do(func() {
        globalSassTranspiler, globalSassTranspilerErr = sass.Start(sass.Options{}) // PATH-only
        globalSassTranspilerErr = sasserrors.Enhance(globalSassTranspilerErr)
    })
    return globalSassTranspiler, globalSassTranspilerErr
}
```

### Code changes required (separate from the installer, ideally land first)

1. Replace our `renderers/sass.go` with upstream's: drop `sassExecutable()` and its
   `os/exec` / `runtime` imports; adopt upstream's `getSassTranspiler()` singleton
   in `renderers/renderers.go` (`sync.Once`, `sass.Start(sass.Options{})`,
   `sasserrors.Enhance`). This also brings the logger usage and the stale-CSS
   cleanup in `copySASSFileIncludes`.
2. **Drop dart-sass bundling from `.goreleaser.yaml`:** remove the
   `bash scripts/download-dart-sass.sh` `before` hook, the `files:` block that
   copied `.dart-sass/...` into archives, and the separate **musl** build/archive
   split (it existed mainly to ship the right dart-sass libc variant). Once the
   code only consults PATH, a `dart-sass/` next to the binary is never read, so
   bundling is dead weight. Result: far smaller archives, fewer artifacts.
   - Keep `scripts/download-dart-sass.sh` in-repo as the reference platform→asset
     mapping (the installer reuses the same mapping), or delete it if unused.

> Note for `go install` users: `go install github.com/reidransom/gojekyll@latest`
> ships no dart-sass (it never did — bundling only lived in release archives), so
> those users must have `sass` on PATH themselves. Document `brew install sass` /
> `npm i -g sass` / the dart-sass releases page in the README.

## What already exists (don't rebuild)

- GoReleaser (`.goreleaser.yaml`) + `.github/workflows/goreleaser.yml` publish
  per-tag GitHub Releases with cross-platform tarballs and a `checksums.txt`.
- gojekyll asset name template (uname-friendly):
  `gojekyll_{title .Os}_{x86_64|arm64|armvN|riscv64}{amd64 variant v1/v2/v3}.tar.gz`
  (the `-musl` archives go away with the change above).
  - macOS: `gojekyll_Darwin_arm64.tar.gz`, `gojekyll_Darwin_x86_64v1.tar.gz`
  - Linux: `gojekyll_Linux_x86_64v1.tar.gz`, `gojekyll_Linux_arm64.tar.gz`
  - Windows: `.zip` (out of scope for a `sh` installer)
- dart-sass releases live at `github.com/sass/dart-sass`; current pinned version is
  **1.98.0** (`scripts/download-dart-sass.sh`). dart-sass asset naming + the
  go-arch→dart-platform map (lift directly from that script):
  | gojekyll platform | dart-sass asset (`dart-sass-<ver>-<platform>.<ext>`) |
  |---|---|
  | darwin arm64 | `macos-arm64` (tar.gz) |
  | darwin amd64 | `macos-x64` (tar.gz) |
  | linux amd64 | `linux-x64` (tar.gz; `linux-x64-musl` on Alpine) |
  | linux arm64 | `linux-arm64` (tar.gz; `-musl` on Alpine) |
  | linux arm | `linux-arm` (tar.gz) |
  | linux riscv64 | `linux-riscv64` (tar.gz) |

## Critical installer detail: dart-sass is a multi-file dir, not a lone binary

A dart-sass tarball extracts to a **directory**, not a single executable:

```
dart-sass/
  sass            ← launcher; resolves its own real path to find src/
  src/dart
  src/sass.snapshot
```

So the installer **cannot** drop a single `sass` into a bin dir. It must install the
whole `dart-sass/` dir and **symlink** `sass` onto PATH:

- Install dir: `$BIN_DIR/dart-sass/` (alongside the gojekyll binary's dir).
- Symlink: `$BIN_DIR/sass -> $BIN_DIR/dart-sass/sass` (the launcher follows the
  symlink to locate `src/`).

Because gojekyll now finds `sass` purely via PATH, `$BIN_DIR` must be on the user's
**runtime** PATH — same PATH hint we already print for the gojekyll binary.

## install.sh design

Location: repo root `install.sh` (served raw from `main`). Plain POSIX `sh`.

Steps:
1. `set -eu`; define `REPO="reidransom/gojekyll"`, `BINARY="gojekyll"`,
   `DART_SASS_VERSION="1.98.0"` (keep in sync with the build; or read from a
   pinned source).
2. Detect OS: `uname -s` → `Linux`/`Darwin`; bail on anything else (point Windows
   users at the releases page / `go install` + `sass` on PATH).
3. Detect arch from `uname -m`:
   - `x86_64|amd64` → gojekyll `x86_64v1`, dart `x64`
   - `arm64|aarch64` → gojekyll `arm64`, dart `arm64`
   - `armv7l` → `armv7` / dart `arm` (Linux)
   - `riscv64` → `riscv64` / dart `riscv64` (Linux)
   - else → clear error.
4. Detect libc on Linux for the dart-sass asset only: musl (Alpine /
   `ldd --version | grep -qi musl`) → append `-musl` to the dart platform. (gojekyll
   itself is `CGO_ENABLED=0` static, so its archive no longer needs a musl variant.)
5. Resolve gojekyll version: default `latest` via
   `https://api.github.com/repos/$REPO/releases/latest` → parse `tag_name`
   (grep/sed, no jq). Override via `VERSION=v1.0.1` env.
6. Pick `BIN_DIR`: `INSTALL_DIR` env override > `$HOME/.local/bin` (no sudo) >
   `/usr/local/bin` (sudo only if needed and a tty exists).
7. `TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT`. Use `curl -fsSL` (fallback `wget`).
8. **gojekyll:** download `gojekyll_{OS}_{ARCH}.tar.gz`; verify against
   `checksums.txt` with `sha256sum`/`shasum -a 256` (warn-and-continue only if no
   sha tool, never silently skip); extract; `install -m 0755 gojekyll
   "$BIN_DIR/gojekyll"`.
9. **dart-sass:** if `sass` already on PATH, **skip** (respect the user's existing
   install). Otherwise download
   `dart-sass-${DART_SASS_VERSION}-${dartPlatform}.tar.gz` from
   `github.com/sass/dart-sass/releases`, extract, `rm -rf "$BIN_DIR/dart-sass"`,
   move the extracted `dart-sass/` to `$BIN_DIR/dart-sass/`, preserve the exec bit
   on `sass`, then `ln -sf "$BIN_DIR/dart-sass/sass" "$BIN_DIR/sass"`.
10. PATH hint: if `$BIN_DIR` not on `$PATH`, print the exact `export PATH=...` line.
11. Verify: run `"$BIN_DIR/gojekyll" version` and `"$BIN_DIR/sass" --version`; echo
    both.

### Robustness / polish
- `curl | sh` safety: document the inspect-first alternative
  (`curl -fsSL …/install.sh -o install.sh; less install.sh; sh install.sh`).
- Idempotent: re-running upgrades both in place.
- Clear prefixed log lines (`gojekyll-install:`), diagnostics to stderr.
- Exit non-zero with actionable messages on every failure path; if a platform has
  no dart-sass asset (exotic arch), install gojekyll anyway and warn that SCSS
  needs `sass` on PATH.

## README changes

```bash
# Quick install (macOS / Linux) — installs gojekyll + dart-sass
curl -fsSL https://raw.githubusercontent.com/reidransom/gojekyll/main/install.sh | sh

# Pin a gojekyll version
curl -fsSL https://raw.githubusercontent.com/reidransom/gojekyll/main/install.sh | VERSION=v1.0.1 sh

# Go developers (must have `sass` on PATH for SCSS, e.g. `brew install sass`)
go install github.com/reidransom/gojekyll@latest
```

## Testing

1. `shellcheck install.sh`.
2. Local dry-run on this Mac (arm64) with `INSTALL_DIR=/tmp/gjtest`: confirm
   `gojekyll`, `dart-sass/`, and the `sass` symlink land in `/tmp/gjtest`; both
   `version` calls work.
3. SCSS render check: with `/tmp/gjtest` first on PATH (and no other `sass`), build
   a small site that imports SCSS — confirm gojekyll's PATH-only lookup finds the
   installed `sass`.
4. "Existing sass" path: with a `sass` already on PATH, confirm the installer skips
   the dart-sass download.
5. GitHub API `latest` parse; `VERSION=` pin; unsupported-arch error path.
6. After the goreleaser change, cut a test tag and confirm archives no longer
   contain `dart-sass/` and the `-musl` artifacts are gone.

## Homebrew (next, enabled by the PATH-only decision)

PATH-only makes a clean `brew install gojekyll` possible with dart-sass pulled in
automatically — no separate `brew install sass`. Homebrew symlinks dependency
binaries into its prefix (on PATH), so gojekyll's PATH-only lookup finds `sass`
with zero config. (Bundling would have violated Homebrew's no-vendored-binaries
norms, so this composes better than the old approach.)

- **Dependency:** declare `depends_on "dart-sass"` — the **homebrew-core**
  `dart-sass` formula (bottled; no extra tap needed). It provides the `sass`
  binary. (Alternative: `sass/sass/sass` from the Dart Sass tap, but that needs a
  `brew tap` first, so prefer core `dart-sass`.)
- **Generation:** GoReleaser writes and pushes the formula via a `brews:` block:
  ```yaml
  brews:
    - repository:
        owner: reidransom
        name: homebrew-tap        # new repo: github.com/reidransom/homebrew-tap
      dependencies:
        - name: dart-sass          # → `depends_on "dart-sass"` in the formula
  ```
- **User flow:**
  ```bash
  brew tap reidransom/tap
  brew install gojekyll            # dart-sass comes along automatically
  ```
- **Setup needed:** a `reidransom/homebrew-tap` repo, and a token in the release
  workflow with push access to it (getting into homebrew-core itself is a high bar
  and not worth it).
- **Caveat:** core `dart-sass` and tap `sass/sass/sass` both install a `sass`
  binary and conflict; a user who already has the tap version may hit a conflict
  when `dart-sass` is pulled in. Document the `dart-sass` choice.

## Out of scope (future)

- Windows `.zip` PowerShell installer (`install.ps1`).
- `.deb`/`.rpm` packages (GoReleaser `nfpms:`).
- amd64 `v2`/`v3` auto-detection (baseline `v1` is the safe default).
- A vanity domain (`get.…dev`) — raw GitHub URL avoids hosting.

## Steps (order)

1. **Code:** adopt upstream `renderers/sass.go` + `getSassTranspiler()` singleton;
   drop `sassExecutable()`. Run `go test ./...`.
2. **Release:** strip dart-sass bundling + musl split from `.goreleaser.yaml`.
3. Write `install.sh` at repo root (gojekyll + dart-sass, per design above).
4. `shellcheck` + local dry-run to `/tmp/gjtest` (incl. SCSS + existing-sass paths).
5. Add the Install section to `README.md` (note the `go install` + `sass`-on-PATH
   caveat).
6. (Optional) add a `shellcheck install.sh` CI step.
7. Commit; the script is live from `main` immediately (reads `latest` from the API).
   The goreleaser/code changes take effect on the next `v*` tag.
