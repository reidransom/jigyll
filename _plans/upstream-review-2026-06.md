# Upstream Review: `osteele/gojekyll` vs `reidransom/gojekyll`

_Date: 2026-06-03 · Branch reviewed: `sync-upstream`_

## Guiding goal for merging

**This repo is striving for Ruby Jekyll compatibility.** The north star when
reconciling with upstream is to match the behavior of original Ruby Jekyll as
closely as possible. Practical consequences:

- **Prefer upstream changes that improve Jekyll fidelity** (Unicode slugs,
  permalink case/`:title` handling, TOC, markdown attributes, math, etc.).
- **Do not maintain fork-only features that are outside the scope of original
  Jekyll** — e.g. the `coming-soon` plugin. These should be dropped from the
  compatibility-focused line, not carried forward. Re-evaluate every fork-only
  feature against this lens before preserving it (see step 4 below).

## Headline

The `upstream` git remote actually points at our **own** fork
(`reidransom/gojekyll`), so this review fetched the real `osteele/gojekyll`
directly. The two histories have diverged hard since the common ancestor
(`2d3a2af`, "Bump liquid", **2025-06-01**):

- **77 commits** upstream that we don't have
- **75 commits** of ours that upstream doesn't have
- Upstream shipped **two releases**: `v0.3.0` and `v0.3.1` (both 2026-02-27/28)

This is **not** a fast-forward. A naive `git merge upstream/main` will produce
serious conflicts in the files both sides rewrote. The useful part of this
review is mapping **where we overlap**.

## What upstream shipped (v0.3.0 + v0.3.1)

**Features**
- Kramdown **TOC** (`{:toc}`, `toc_levels`, `{:.no_toc}`) — large new subsystem
- **Math** support (MathJax/KaTeX passthrough)
- Full **markdown attributes** (`markdown=1/0/block/span` in HTML blocks)
- `--baseurl` / `--config` CLI flags
- `sassify` filter (indented Sass)
- New plugins: **jekyll-relative-links**, **README→index remap**, gist `noscript`
- Build diagnostics for skipped files

**Notable fixes**
- Unicode slugs, permalink case preservation, `:title` uses filename slug,
  page-vs-post permalinks
- HTML void elements in markdown blocks, indented HTML rendered as code
- `page.date` undefined for non-posts
- SCSS "connection shut down" (sass singleton)
- Symlink preservation in `_site`
- URL routing without trailing slashes
- Collect-all-render-errors instead of stopping at first

**Infra**
- Centralized **logger** package (replaces scattered `fmt.Printf`)
- Watcher **polling fallback** above 500 dirs
- `log.Fatal` → `panic`/`fmt.Errorf`
- Go 1.25 + golangci-lint v2
- liquid **v1.8.1**; goreleaser v2

## Conflict zones (our 75 commits vs their 77)

| Area | Ours | Upstream | Merge risk |
|---|---|---|---|
| **Markdown engine** | Switched to **goldmark** (`62df384`); kramdown attrs via `renderers/ial.go`; GH source blocks | Also **goldmark** (1.7.13) but kramdown attrs/TOC via `markdown_attrs.go`+`markdown_toc.go`+`markdown_utils.go`; still uses blackfriday in `filters.go` | **High** |
| **Permalinks** | `pretty permalink fix`, date-sort, `f9614d3` post-only custom permalinks | Heavy rewrite of `pages/permalinks.go` (Unicode, case, `:title`, page permalinks) | **High** |
| **File watching** | `fix fsnotify watching`, `fix serve/build looping` | Reworked `site/watch.go` + `server/watcher.go` + polling fallback | **Med-High** |
| **Liquid** | Pinned to **`reidransom/liquid`** fork (v1.8.2-pre) for a `time.Time` sort fix via `replace` | Plain `v1.8.1` | **Med** — `go.mod` conflict; confirm our sort fix is still needed / upstream it |
| **Logging / config** | env-based config, `--admin`, removed `_admin.yml` | New `logger/` package replacing `fmt.Printf` | **Med** |
| **Sass / release** | Bundle dart-sass in archives, musl builds (`.goreleaser`) | sass singleton fix + `sassify` | **Med** (goreleaser + `sass.go`) |
| **Filters** | added `uniq`, `limit`, `where` fix | implemented `sassify` (was stubbed) | **Low** (additive) |
| **Plugins** | added `coming-soon`, `inherit-frontmatter` | added `relative-links`, `readme-index`, default-layout tests | **Low** (additive; built-in registry names match) |

**Key insight:** both sides independently moved to **goldmark**, so the hardest
architectural decision already agrees. But kramdown attributes/TOC were
implemented in *different files with different approaches*, so they'll fight in
the same package.

## Recommended strategy

Don't do a single big merge. Instead:

1. **Cherry-pick low-conflict wins first** — `sassify`, gist `noscript`,
   relative-links/readme-index plugins, build diagnostics, `--baseurl`/`--config`,
   void-element fix, symlink preservation. Mostly additive.
2. **Reconcile permalinks deliberately** — diff `pages/permalinks.go` and
   `permalinks_test.go` both ways; upstream's Unicode/case/`:title` fixes likely
   supersede or complement our `pretty permalink fix`. Run both test suites.
3. **Reconcile markdown** — decide whether to adopt upstream's
   `markdown_attrs.go`/`markdown_toc.go` (better-tested, TOC + math) and retire
   our `ial.go`, or keep ours. Probably adopt upstream's since it's broader.
4. **Re-evaluate fork-only features against the Jekyll-compatibility goal** —
   - **Drop** features outside original Jekyll's scope: **`coming-soon`** is the
     clearest case and should be removed.
   - **Keep** anything that aids Jekyll-compatible deployment/build but is
     genuinely neutral to the core feature set (e.g. musl/dart-sass bundling).
   - **Judgment calls** (`--admin`, env-based config, `inherit-frontmatter`):
     keep only if they don't diverge from Jekyll semantics; otherwise drop or
     gate them. `inherit-frontmatter` mirrors a real Jekyll pattern (defaults),
     so likely keep; `--admin` is fork-specific tooling — keep only if still
     needed operationally.
5. **Liquid** — try upstream `v1.8.1` directly; if our `time.Time` sort fix isn't
   in it, upstream that one-line fix rather than carrying a `replace` fork forever.

## Reference

- Common ancestor: `2d3a2aff4032db5f8faf0582e2674e1288f9dd52` (2025-06-01)
- Upstream tip reviewed: `f975f30` (2026-02-28, "Merge PR #127 fix-github-metadata-panic")
- To re-fetch upstream: `git fetch https://github.com/osteele/gojekyll.git main`
