# Plan: verify the Homebrew install

How to confirm `brew install gojekyll` works end-to-end after the `brews:` block
was added to `.goreleaser.yaml`. Work top to bottom; the local dry-run (step 2)
catches most problems before you ever cut a tag.

Context:
- Source repo: `reidransom/gojekyll`
- Tap repo: `reidransom/homebrew-tap` (formula pushed to `Formula/gojekyll.rb`)
- Formula declares `depends_on "dart-sass"` (homebrew-core) → provides `sass` on PATH
- gojekyll resolves dart-sass from PATH only (PATH-only Sass change)

## 0. Prerequisite (one-time): the tap token

The release workflow needs `HOMEBREW_TAP_GITHUB_TOKEN` or the brew step silently
won't publish.

1. Create a **fine-grained PAT** scoped to `reidransom/homebrew-tap`,
   **Contents: read/write**.
2. Add it as a secret on the source repo:
   ```bash
   gh secret set HOMEBREW_TAP_GITHUB_TOKEN --repo reidransom/gojekyll
   # paste the PAT
   ```
3. Confirm it's there:
   ```bash
   gh secret list --repo reidransom/gojekyll | grep HOMEBREW_TAP_GITHUB_TOKEN
   ```

## 1. Static config sanity

```bash
brew install goreleaser            # if not present
HOMEBREW_TAP_GITHUB_TOKEN=dummy goreleaser check
```
Expect "1 configuration file(s) validated". The `brews is being phased out`
deprecation line is expected and fine (we keep `brews` for Linux support).

## 2. Local dry-run — inspect the generated formula WITHOUT releasing

This builds artifacts and renders the formula locally; nothing is pushed.

```bash
HOMEBREW_TAP_GITHUB_TOKEN=dummy goreleaser release --snapshot --clean --skip=publish
```

Then inspect the generated formula:

```bash
find dist -name '*.rb' -exec cat {} +
```

Check the rendered `gojekyll.rb` for:
- [ ] `depends_on "dart-sass"` present
- [ ] `url`s point at `github.com/reidransom/gojekyll/releases/download/...`
- [ ] separate `on_macos`/`on_linux` + `on_arm`/`on_intel` blocks with the right
      archive names (`gojekyll_Darwin_arm64.tar.gz`, `gojekyll_Linux_x86_64v1.tar.gz`, …)
- [ ] `sha256` values populated (not blank)
- [ ] `bin.install "gojekyll"` and the `test do … "version" … end` block
- [ ] amd64 resolves to the **v1** archive (Homebrew default), not v2/v3

If anything looks off, fix `.goreleaser.yaml` and re-run step 2 before tagging.

### Optional: install straight from the dry-run formula

```bash
brew install --build-from-source --formula dist/gojekyll.rb   # or:
brew install --formula ./dist/.../gojekyll.rb
```
Then run the post-install checks in step 4. (Local-file installs skip the tap/URL
plumbing, so still do a real tagged release before trusting it.)

## 3. Cut a release (the real path)

Tag and push; the `goreleaser` workflow runs on `v*` tags.

```bash
git checkout main && git pull          # ensure curl-installer work is merged first
git tag v1.0.2 && git push origin v1.0.2
gh run watch --repo reidransom/gojekyll   # follow the release job
```

After it goes green, confirm the formula landed in the tap:

```bash
gh api repos/reidransom/homebrew-tap/contents/Formula/gojekyll.rb \
  --jq '.name + " @ " + .sha'
# or just open https://github.com/reidransom/homebrew-tap/tree/main/Formula
```

## 4. Verify the install on macOS

```bash
brew untap reidransom/tap 2>/dev/null || true   # clean slate
brew tap reidransom/tap
brew install gojekyll
```

Checks:
- [ ] `brew deps gojekyll` lists `dart-sass`
- [ ] `which gojekyll` → under the Homebrew prefix (`/opt/homebrew/bin` or `/usr/local/bin`)
- [ ] `which sass` resolves (pulled in by the dependency)
- [ ] `gojekyll version` runs
- [ ] **SCSS render** through the brew-installed toolchain:
  ```bash
  mkdir -p /tmp/brewscss/css
  printf 'title: t\n' > /tmp/brewscss/_config.yml
  printf -- '---\n---\n$c:#f00; body{color:$c; .x{margin:1px + 2px}}\n' > /tmp/brewscss/css/style.scss
  gojekyll build -s /tmp/brewscss -d /tmp/brewscss/_site
  cat /tmp/brewscss/_site/css/style.css   # expect: body{color:red}body .x{margin:3px}
  ```
- [ ] `brew audit --strict --online gojekyll` (style/URL/dep issues; warnings are
      tolerable for a personal tap, but read them)

## 5. Verify on Linux (optional but recommended)

The whole reason we kept a formula over a cask is Linux support — so confirm it.
Easiest via the Homebrew/brew container:

```bash
docker run --rm -it homebrew/brew bash -lc '
  brew tap reidransom/tap &&
  brew install gojekyll &&
  gojekyll version &&
  which sass'
```
Expect gojekyll + dart-sass to install and `gojekyll version` to run.

## 6. Re-test cleanly (between attempts)

```bash
brew uninstall gojekyll
brew untap reidransom/tap
rm -rf /tmp/brewscss
```

To test a new formula version, bump the tag (`v1.0.3`, …) — Homebrew caches by
version, so reusing a tag won't pick up changes.

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| Workflow green but tap has no/old `gojekyll.rb` | `HOMEBREW_TAP_GITHUB_TOKEN` missing or lacks Contents:write on the tap (step 0) |
| `goreleaser` errors "multiple archives match" for amd64 | brew can't pick an amd64 variant — set `goamd64: v1` under the `brews:` entry |
| `brew install` fails: dependency `dart-sass` not found | depending on the tap `sass/sass/sass` instead of core `dart-sass`; keep core |
| Installs but SCSS fails: `exec: "sass": not found` | `dart-sass` dep didn't link `sass` onto PATH; check `brew link dart-sass` / `which sass` |
| macOS "cannot be opened/verified" on the binary | only an issue for casks of unsigned binaries — N/A for a formula; if it appears, the artifact is being shipped as a cask by mistake |
| `brew audit` flags the URL/license | cosmetic for a personal tap; fix `homepage`/`license`/description in `.goreleaser.yaml` if you care |

## Done = all true

- Tagged release is green; `Formula/gojekyll.rb` present in the tap at the new version.
- `brew install gojekyll` on macOS pulls in `dart-sass`, `gojekyll version` runs,
  and the SCSS sample compiles to `body{color:red}body .x{margin:3px}`.
- (Optional) Same succeeds in the `homebrew/brew` Linux container.
