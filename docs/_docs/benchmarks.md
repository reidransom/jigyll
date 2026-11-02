---
title: Benchmarks
permalink: /docs/benchmarks/
description: Build-time comparisons between Ruby Jekyll and Jigyll on real sites.
---

This page lists the times to run `jekyll build` and `jigyll build` on a few
different sites. See the Benchmarks section of
[CONTRIBUTING.md](https://github.com/reidransom/jigyll/blob/main/CONTRIBUTING.md)
for instructions on how to run a benchmark.

Notes:

- Sass conversion is cached.
- Pygments (for versions ≤ 0.2.5) is cached.

## MadelineProto Docs

`jekyll build` vs `jigyll build` on an Intel Xeon E5620 @ 2.40GHz, running
current versions of everything as of 2022-01-29.

This site contains 1873 Markdown files, and runs a modified version of the
complex [Just The Docs theme](https://pmarsceill.github.io/just-the-docs/),
with many Sass files, sitemap, and search index generation.

| Executable | Options         | Time             |
| ---------- | --------------- | ---------------- |
| jekyll     |                 | Timeout @ 1 hour |
| jigyll     | single-threaded | 750.61s          |
| jigyll     | multi-threaded  | 142.16s          |

## Software Design web site

Site source: <https://github.com/sd17spring/sd17spring.github.io>

MacBook Pro (13", M1, 2020), macOS Monterey (12.2), jigyll v0.2.5,
go1.17.6 darwin/arm64.

Notes:

- This site makes heavy use of Sass.
- This site uses the GitHub metadata plugin. The results of that plugin are
  not cached. (See issue
  [gojekyll#43](https://github.com/osteele/gojekyll/issues/43).) These
  benchmarks were run from a machine with a high latency to GitHub; that
  latency likely dominates these times. Previous benchmarks were from the
  U.S.

| Executable | Options                         | Time               |
| ---------- | ------------------------------- | ------------------ |
| jigyll     | single-threaded; cache disabled | 1.568 s ±  0.145 s |
| jigyll     | single-threaded; warm cache     | 1.427 s ±  0.191 s |
| jigyll     | multi-threaded; cache disabled  | 1.291 s ±  0.104 s |
| jigyll     | multi-threaded; warm cache      | 1.118 s ±  0.110 s |
