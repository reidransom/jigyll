# Plan: verify the Scoop install (Windows)

How to confirm `scoop install jigyll` works end-to-end after the `scoops:` block
was added to `.goreleaser.yaml`. This mirrors the Homebrew verification
(`completed/verify-brew-install.md`) — same shape, Windows package manager.
Work top to bottom; the local dry-run (step 2) catches most problems before you
cut a tag, and it runs on **any** OS. Only steps 4+ need real Windows.

Context:
- Source repo: `reidransom/jigyll`
- Bucket repo: `reidransom/scoop-bucket` (manifest pushed to `bucket/jigyll.json`)
- Manifest declares `"depends": ["main/sass"]` → provides `sass` on PATH.
  NOTE: Scoop has **no** `dart-sass` package — the main-bucket package is named
  `sass` (it *is* Dart Sass; `dart-sass` is only a binary alias). Confirmed via
  `scoop search dart-sass` → matches `sass` (main) by its binary alias.
- jigyll resolves sass from PATH only (same PATH-only Sass behaviour as brew)
- Scoop maps amd64→`64bit` (Windows `x86_64v1.zip`) and arm64→`arm64` automatically

## 0. Prerequisites (one-time)

The release workflow needs a token AND a bucket repo, or the scoop step silently
won't publish (exactly like the brew `401 Bad credentials` failure on v1.0.3).

1. **Create the bucket repo** `reidransom/scoop-bucket` (public, empty is fine —
   GoReleaser creates `bucket/jigyll.json` on first release). A Scoop "bucket" is
   the Windows analogue of a Homebrew tap.
   ```bash
   gh repo create reidransom/scoop-bucket --public \
     --description "Scoop bucket for jigyll and friends"
   ```
2. **Create a fine-grained PAT** scoped to `reidransom/scoop-bucket`,
   **Contents: read/write**.
3. Add it as a secret on the source repo:
   ```bash
   gh secret set SCOOP_BUCKET_GITHUB_TOKEN --repo reidransom/jigyll
   # paste the PAT
   ```
4. Confirm it's there:
   ```bash
   gh secret list --repo reidransom/jigyll | grep SCOOP_BUCKET_GITHUB_TOKEN
   ```

> ⚠️ Lesson from the brew run: add the token **before** tagging. v1.0.3 released
> before `HOMEBREW_TAP_GITHUB_TOKEN` existed and the brew step died with 401. Get
> step 0 fully done first.

## 1. Static config sanity (any OS)

```bash
HOMEBREW_TAP_GITHUB_TOKEN=dummy SCOOP_BUCKET_GITHUB_TOKEN=dummy goreleaser check
```
Expect "1 configuration file(s) validated". The `brews is being phased out`
deprecation line is unrelated and fine.

## 2. Local dry-run — inspect the generated manifest WITHOUT releasing (any OS)

This builds artifacts and renders the manifest locally; nothing is pushed.

```bash
HOMEBREW_TAP_GITHUB_TOKEN=dummy SCOOP_BUCKET_GITHUB_TOKEN=dummy \
  goreleaser release --snapshot --clean --skip=publish
cat dist/scoop/bucket/jigyll.json
```

Check the rendered `jigyll.json` for:
- [ ] `"depends": ["main/sass"]` present
- [ ] `architecture.64bit.url` → `.../releases/download/vX.Y.Z/jigyll_Windows_x86_64v1.zip`
      (the **v1** archive — Scoop's amd64 default, not v2/v3)
- [ ] `architecture.arm64.url` → `jigyll_Windows_arm64.zip`
- [ ] each arch has `"bin": ["jigyll.exe"]`
- [ ] `hash` values populated (not blank)
- [ ] `homepage` / `license` / `description` correct

If anything looks off, fix `.goreleaser.yaml` and re-run step 2 before tagging.

## 3. Cut a release (the real path)

Tag and push; the `goreleaser` workflow runs on `v*` tags and now publishes BOTH
the brew formula and the scoop manifest.

```bash
git checkout main && git pull
git tag v1.0.5 && git push origin v1.0.5
gh run watch --repo reidransom/jigyll   # follow the release job
```

After it goes green, confirm the manifest landed in the bucket:

```bash
gh api repos/reidransom/scoop-bucket/contents/bucket/jigyll.json \
  --jq '.name + " @ " + .sha'
```

## 4. Verify the install on Windows

Needs a real Windows box (or VM / GitHub Actions `windows-latest` runner). In
PowerShell:

```powershell
# install scoop itself if needed:
#   Set-ExecutionPolicy -Scope CurrentUser RemoteSigned
#   irm get.scoop.sh | iex

scoop bucket add reidransom https://github.com/reidransom/scoop-bucket
scoop install jigyll
```

Checks:
- [ ] `scoop install jigyll` pulls in `sass` (Dart Sass) as a dependency
- [ ] `where.exe jigyll` → under the Scoop shims dir (`~\scoop\shims`)
- [ ] `where.exe sass` resolves (pulled in by the dependency)
- [ ] `jigyll version` runs
- [ ] **SCSS render** through the scoop-installed toolchain:
  ```powershell
  mkdir C:\tmp\scss\css -Force
  "title: t"                                            | Out-File -Encoding ascii C:\tmp\scss\_config.yml
  "---`n---`n`$c:#f00; body{color:`$c; .x{margin:1px + 2px}}" | Out-File -Encoding ascii C:\tmp\scss\css\style.scss
  jigyll build -s C:\tmp\scss -d C:\tmp\scss\_site
  Get-Content C:\tmp\scss\_site\css\style.css   # expect: body{color:red}body .x{margin:3px}
  ```

> The dependency is `main/sass`, not `dart-sass` — Scoop's `sass` package *is*
> Dart Sass and shims `sass` onto PATH. Verified with `scoop search dart-sass`,
> which returns `sass` (main) matched on its `dart-sass` binary alias, plus an
> unrelated `dart-sass-embedded` package we do **not** want.

## 5. Re-test cleanly (between attempts)

```powershell
scoop uninstall jigyll
scoop bucket rm reidransom
Remove-Item -Recurse -Force C:\tmp\scss
```

To test a new manifest version, bump the tag (`v1.0.6`, …) — Scoop caches by
version, so reusing a tag won't pick up changes.

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| Workflow green but bucket has no/old `jigyll.json` | `SCOOP_BUCKET_GITHUB_TOKEN` missing or lacks Contents:write on the bucket (step 0) — same failure mode as the brew 401 |
| Release fails: bucket repo not found | `reidransom/scoop-bucket` doesn't exist yet (step 0.1) |
| `scoop install` can't resolve the sass dep | use `main/sass` (NOT `dart-sass`, which isn't a Scoop package); ensure the `main` bucket is added |
| Installs but SCSS fails: `exec: "sass": not found` | `sass` shim not on PATH; check `scoop install sass` / `where.exe sass` |
| 64bit url points at v2/v3 zip | set `goamd64: v1` under the `scoops:` entry (Scoop wants one amd64 variant) |

## Done = all true

- Tagged release is green; `bucket/jigyll.json` present in `reidransom/scoop-bucket`
  at the new version.
- `scoop install jigyll` on Windows pulls in `dart-sass`, `jigyll version` runs,
  and the SCSS sample compiles to `body{color:red}body .x{margin:3px}`.
