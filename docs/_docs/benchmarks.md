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

## Local comparison: November 7, 2026

The [landing page]({{ '/' | relative_url }}) shows observed build times from two
local runs, not a general speed guarantee. Each command had 10 measured runs after
two warmups. Both engines passed semantic output preflight. The generated fixture
contains 2,000 posts and 20 standalone pages plus the index, 10 includes and four
layouts; it has no Sass. The small Sass workload is the pinned, prepared
`jekyll-theme-console` site described below, with 14 output files.

The host was Linux x86-64 with an AMD Ryzen 7 255 and 16 logical processors.
Jigyll was compiled from clean commit
[`1c5e8ae`](https://github.com/reidransom/jigyll/commit/1c5e8ae1bd5ca95e249cc75a6de0bb8fd1c64d29)
using Go 1.26.1. Tools: Ruby 3.4.7, Bundler 2.6.9, Jekyll 4.4.1,
Hyperfine 1.19.0 and Dart Sass/sass-embedded 1.102.0.

The generated run is `run-wBCAKx6m`; the Sass run is `run-eSEEEQP7`.
Their raw samples and provenance are retained locally under the artifact root
above, not served by this documentation site. The Sass diagnostic rows reuse one
unchanged Ruby baseline; only Jigyll's cache/concurrency settings change.
Default settings use warm persistent caches. Disabling cache reuse does not
disable every cache or eliminate cache I/O.

The measurement includes fresh process startup (including Bundler for Ruby) and
native output cleanup/writes, while excluding setup and compilation. Startup and
cache effects are especially relevant to the small Sass site: its default ratio
was 35.17× in the displayed run and 31.73× in a separate rerun. The default
generated-site ratio was 4.70×.

These runs were not quiet-host, reversed-order experiments. The generated run's
one-minute host load rose from 2.92 to 5.22. Ratios describe these samples, not
statistical significance or performance on other machines. For more stable
estimates, repeat both command orders on a quiet host and retain all artifacts;
disclose noise and order sensitivity rather than choosing the fastest result.

### Measured build times

Mean ± sample standard deviation, **10 runs after two warmups**. Diagnostic
settings change **Jigyll only**; all Sass rows reuse the same unchanged Ruby
Jekyll measurement. “Faster” is the ratio of mean Jekyll time to mean Jigyll time.

| Workload / Jigyll configuration | Ruby Jekyll | Jigyll | Jekyll time ÷ Jigyll time |
| --- | ---: | ---: | ---: |
| Generated: 2,000 posts, 20 pages; default | 4.308 ± 0.115 s | 0.918 ± 0.014 s | **4.70×** |
| Small Sass site; default | 359.5 ± 13.1 ms | 10.22 ± 0.52 ms | **35.17×** |
| Small Sass site; single processor | 359.5 ± 13.1 ms | 11.04 ± 0.67 ms | **32.56×** |
| Small Sass site; cache reuse disabled | 359.5 ± 13.1 ms | 35.18 ± 3.09 ms | **10.22×** |
| Small Sass site; single processor, cache reuse disabled | 359.5 ± 13.1 ms | 32.51 ± 1.55 ms | **11.06×** |

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
