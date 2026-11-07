---
title: Benchmarks
permalink: /docs/benchmarks/
description: Reproduce native full-build comparisons and understand historical upstream Gojekyll measurements.
---

## Measure the current Jigyll checkout

Run `just perf-setup` followed by `just perf` from the checkout root. This prepares
pinned native tools and measures 10 full builds after two warmups on a deterministic
2,000-post fixture. See the [contributor workflow](https://github.com/reidransom/jigyll/blob/main/CONTRIBUTING.md#benchmarks)
for prerequisites, installation paths, all profiles, existing-site bundles,
fairness controls and artifact interpretation.

The harness compares a precompiled Jigyll CLI with native Bundler/Jekyll startup
and a full build, including filesystem writes. It excludes compilation, dependency
installation, fixture generation and preflight. Every run retains raw samples,
verification logs, commands, statistics and provenance under
`.scratch/performance-comparison/runs/run-*/`. Failures invalidate the comparison.
The ratio is explicitly `mean(Jekyll) / mean(Jigyll)`; below one means Jigyll is
slower. Diagnostic profiles change Jigyll only, not the Ruby baseline.

Local functional verification exercised the full default and all diagnostic
profiles, plus the prepared Sass site below. Those runs were not quiet-host,
reversed-order publication experiments. This page makes **no current Jigyll
speed claim**. Repeat both command orders on a quiet host and retain all artifacts
before publishing timings; disclose noise and order sensitivity.

## Verified real-site preparation

The compatible Sass workload is the theme-development/demo site from
[b2a3e8/jekyll-theme-console](https://github.com/b2a3e8/jekyll-theme-console/tree/00d968213ea0022416da15c5575119cd08e58757),
pinned at `00d968213ea0022416da15c5575119cd08e58757`. It is a **prepared comparison
input**, not an unchanged historical baseline. All upstream templates, content,
includes, assets and five Sass entrypoints are retained. Both engines receive the
same pinned dependency constraints, disabled Sass source maps and non-site excludes.

After `just perf-setup`, use Bash from the repository root. Choose a new private
site directory; do not append this preparation repeatedly to an existing site.

```bash
site="$PWD/.scratch/performance-comparison/console site"
git clone https://github.com/b2a3e8/jekyll-theme-console.git "$site"
git -C "$site" checkout --detach 00d968213ea0022416da15c5575119cd08e58757
cat >> "$site/Gemfile" <<'GEMS'

gem "jekyll", "= 4.4.1"
gem "sass-embedded", "= 1.102.0"
GEMS
cat >> "$site/_config.yml" <<'CONFIG'

sass:
  sourcemap: never
exclude: [Gemfile, Gemfile.lock, vendor, README.md, LICENSE.txt]
CONFIG
cp benchmarks/Gemfile.lock "$site/Gemfile.lock"
(
  source scripts/perf-env
  export BUNDLE_GEMFILE="$site/Gemfile"
  unset BUNDLE_FROZEN
  bundle _2.6.9_ lock
)
scripts/perf-setup "$site"
just perf "$site" 10 2 all
```

The original revision has no lockfile. The seed lock pins the comparison baseline;
resolving the site's additional dependencies happens **outside timing**. Save the
resulting Gemfile, lock, configuration changes and source revision together.
Future resolutions may choose different transitive dependencies: exact replay
requires the retained resolved lock, not another fresh resolution. Setup then
installs that frozen bundle in a private content-keyed directory and never edits
the site's lock. The verified local prepared lock has SHA-256
`ac7d9820dc126dd4f8cac74fcc20fa7f51c5fe7cafd92eacdef6aeb21fc34d1a`.

Local verification checked matching 14 output paths, normalized text/navigation
in both HTML pages, equivalent CSS and byte-equal other assets. Both engines
compiled `assets/main.scss`, `main-dark.scss`, `main-hacker.scss`, `main-light.scss`
and `main-nord.scss`. Timing used local imports and a frozen installed bundle,
without build-time downloads. The generated workload has no Sass and cannot
establish Sass-cache benefits; disabling Jigyll cache reuse still performs cache I/O.

Two other pinned candidates were not valid comparisons:

* Software Design at `9db166478843ac0c74c44ac141db8f12840e53fb` retains a historical
  GitHub Pages 138/Jekyll 3.4.3 lock. Its Nokogiri 1.7.2 native extension failed on
  Ruby 3.4.7; its GitHub metadata plugin also makes network latency relevant.
  No current timing is claimed and the original lock was not rewritten.
* Forty at `c3447cf3a32b6cd1a3c9887ab550e8a8810899ce` failed semantic preflight:
  Jigyll omitted six posts with unpadded dates that Ruby emitted. The harness
  rejected the unequal work; posts were not renamed or removed to force success.

## Historical upstream Gojekyll measurements

The following tables are inherited **Gojekyll**, not current Jigyll, measurements.
They retain the historical evidence from
[upstream's benchmark page](https://github.com/osteele/gojekyll/blob/4d08286474a1d218387808d7d7c695b2cc01843a/docs/benchmarks.md).
The published history does not provide a complete locked dependency set, source
revision and raw samples for every table. Do not treat these as a reproducible
measurement of today's checkout. Historical notes say Sass conversion and
Pygments (versions ≤ 0.2.5) were cached.

### MadelineProto Docs (2022)

Historical `jekyll build` versus `gojekyll build` on an Intel Xeon E5620 @ 2.40GHz,
with versions current as of 2022-01-29. The site contained 1,873 Markdown files
and a modified [Just the Docs theme](https://pmarsceill.github.io/just-the-docs/),
with Sass, sitemap and search-index generation.

| Executable | Options | Time |
| --- | --- | --- |
| Jekyll | | Timeout at 1 hour |
| Gojekyll | Single-threaded | 750.61 s |
| Gojekyll | Multi-threaded | 142.16 s |

The timeout is a censored observation, not an exact Jekyll duration or speedup.

### Software Design web site (historical)

Site source: <https://github.com/sd17spring/sd17spring.github.io>.
Historical environment: MacBook Pro (13-inch, M1, 2020), macOS Monterey 12.2,
Gojekyll v0.2.5 and Go 1.17.6 darwin/arm64.

The site uses Sass and the GitHub metadata plugin. Metadata requests were not
cached ([upstream issue 43](https://github.com/osteele/gojekyll/issues/43)); the
upstream author reported high GitHub latency that likely dominated these times.

| Executable | Options | Time |
| --- | --- | --- |
| Gojekyll | Single-threaded; cache disabled | 1.568 s ± 0.145 s |
| Gojekyll | Single-threaded; warm cache | 1.427 s ± 0.191 s |
| Gojekyll | Multi-threaded; cache disabled | 1.291 s ± 0.104 s |
| Gojekyll | Multi-threaded; warm cache | 1.118 s ± 0.110 s |
