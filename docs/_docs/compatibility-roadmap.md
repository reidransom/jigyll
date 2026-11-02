---
title: GitHub Pages Compatibility Roadmap
permalink: /docs/compatibility-roadmap/
description: A phased plan for verifying Jigyll against GitHub Pages, Jekyll 3.10, Jekyll 4.4, and the supported Pages plugins.
---

Jigyll compatibility should be demonstrated by reproducible comparisons, not
inferred from similar-looking output. This roadmap establishes Ruby Jekyll as
the reference implementation, closes the known GitHub Pages plugin gaps, and
keeps the comparison current when GitHub changes its build environment.

“Support” here means that a fixture produces the same public site behavior:
routes, rendered content, generated files, metadata, redirects, warnings, and
build failures. Byte-for-byte equality is the default gate. A difference may be
normalized only when it is inherently nondeterministic, such as a generated
timestamp or live GitHub API response, and every normalization must be named in
the comparison report.

## Target profiles

The profile is the compatibility contract. Ruby itself is part of the
reproducible reference environment, but Jigyll emulates the resulting Jekyll
behavior; it does not embed or execute Ruby.

| Priority | Profile | Reference environment | Purpose |
| --- | --- | --- | --- |
| P0 | GitHub Pages | Ruby 3.3.4, `github-pages` 232, Jekyll 3.10.0, and the exact [Pages dependency set][pages-versions] | Match GitHub's branch-based hosted builder |
| P1 | Jekyll 3.10 | Ruby 3.3.4 and standalone Jekyll 3.10.0 | Separate Jekyll behavior from Pages defaults and plugins |
| P1 | Current Jekyll | Ruby 3.3.4 and standalone Jekyll 4.4.1 | Cover the current Jekyll 4 behavior used by custom Actions builds |
| P2 | Historical Pages | Tagged `github-pages` releases that changed the Jekyll minor version | Detect compatibility boundaries without testing every equivalent patch release |

The P0 manifest must be refreshed from
[`pages.github.com/versions.json`][pages-versions-json]. A dependency change
creates a new immutable profile; it must not silently rewrite existing expected
output. Historical profiles should be selected from the [Pages gem release
history][pages-releases] after the P0 and P1 profiles pass.

## Current plugin baseline

GitHub's published dependency set contains 19 plugin or converter gems. The
status below records Jigyll's documented starting point; completion requires the
comparison suite, even where the current implementation is marked complete.

| GitHub Pages plugin | Pages version | Current Jigyll status | Roadmap work |
| --- | ---: | --- | --- |
| `jekyll-avatar` | 0.8.0 | Complete | Add GitHub Pages output fixtures |
| `jekyll-coffeescript` | 1.2.2 | Missing | Implement asset discovery, compilation, and failure behavior |
| `jekyll-commonmark-ghpages` | 0.5.1 | Partial | Honor Markdown processor selection and CommonMark options |
| `jekyll-default-layout` | 0.1.5 | Complete | Verify layout precedence and collection scope |
| `jekyll-feed` | 0.17.0 | Complete | Verify feed content, paths, excerpts, and configuration |
| `jekyll-gist` | 1.5.0 | Complete | Verify tag arguments, filenames, and script fallback markup |
| `jekyll-github-metadata` | 2.16.1 | Partial | Add missing repository fields and deterministic API fixtures |
| `jekyll-include-cache` | 0.2.1 | Complete | Verify cache keys, parameters, includes, and invalidation |
| `jekyll-mentions` | 1.6.0 | Complete | Verify exclusions, URL prefixes, and HTML boundaries |
| `jekyll-optional-front-matter` | 0.3.2 | Complete | Verify extension and excluded-name rules |
| `jekyll-paginate` | 1.1.0 | Complete | Verify paginator drops, page paths, and invalid configuration |
| `jekyll-readme-index` | 0.3.0 | Complete | Verify index precedence, extensions, and front matter |
| `jekyll-redirect-from` | 0.16.0 | Partial | Match the default template and redirect route behavior |
| `jekyll-relative-links` | 0.6.1 | Complete | Verify collections, index files, and disabled rendering |
| `jekyll-remote-theme` | 0.4.3 | Partial | Resolve pinned GitHub `owner/repository@40-character-SHA` archives; branches, tags, omitted refs, arbitrary URLs, and full plugin behavior remain unsupported |
| `jekyll-seo-tag` | 2.8.0 | Partial | Complete publisher, author, image, social, and canonical metadata |
| `jekyll-sitemap` | 1.4.0 | Partial | Match file modification dates and exclusion rules |
| `jekyll-titles-from-headings` | 0.5.3 | Missing | Implement title extraction, stripping, and configuration |
| `jemoji` | 0.13.0 | Partial | Match GitHub's image fallback and HTML-safe replacement behavior |

GitHub's user-facing documentation identifies nine plugins as always enabled:
`jekyll-coffeescript`, `jekyll-default-layout`, `jekyll-gist`,
`jekyll-github-metadata`, `jekyll-optional-front-matter`, `jekyll-paginate`,
`jekyll-readme-index`, `jekyll-titles-from-headings`, and
`jekyll-relative-links`. The Pages profile must activate these without requiring
a `plugins:` entry. Other supported plugins must activate when configured, and
unsupported plugin names must produce a clear warning or error matching the
selected profile.

## Phase 1 — Build the comparison oracle

- Define immutable reference profiles
  - Create a machine-readable manifest for Ruby, Jekyll, `github-pages`, and every plugin version.
  - Pin a container image by digest for each Ruby reference environment.
  - Record the source URL and retrieval date beside each generated manifest.
- Build a shared fixture corpus
  - Create one minimal fixture for every core rendering behavior.
  - Create one isolated fixture for every supported plugin.
  - Create interaction fixtures for plugins that modify the same pages or drops.
  - Add fixtures for expected warnings and build failures.
- Produce comparable artifacts
  - Build each fixture with Ruby Jekyll into a profile-specific output directory.
  - Build the same fixture with Jigyll into a separate output directory.
  - Compare route manifests before comparing file contents.
  - Compare text files structurally and binary files by digest.
  - Emit a machine-readable report containing every mismatch and normalization.
- Make external inputs deterministic
  - Replace GitHub API calls with recorded responses for metadata tests.
  - Use local repositories or archives for remote-theme tests.
  - Freeze time only for fixtures whose reference plugin reads the clock.
- Integrate the oracle
  - Add one command that runs a named fixture against a named profile.
  - Add one command that runs the complete P0 compatibility suite.
  - Cache Ruby gems and reference images in CI.
  - Upload the comparison report when a compatibility job fails.

**Exit gate:** the harness detects deliberate route, HTML, metadata, generated
file, warning, and failure mismatches. The report contains no unnamed ignored
paths or blanket HTML normalization.

## Phase 2 — Match the GitHub Pages runtime

- Pin the current Pages contract
  - Capture the P0 dependency manifest from the official versions endpoint.
  - Match Pages-forced Jekyll settings such as safe mode, Rouge, and disabled incremental builds.
  - Match the Pages plugin activation list and activation order.
  - Separate standalone Jekyll defaults from GitHub Pages overrides.
- Close always-enabled plugin gaps
  - Implement `jekyll-coffeescript` compilation and error reporting.
  - Implement `jekyll-titles-from-headings` configuration and title precedence.
  - Complete the documented `jekyll-github-metadata` fields.
  - Verify the six already-emulated default plugins against their pinned versions.
- Close core Jekyll 3.10 gaps exposed by the corpus
  - Fix configuration parsing before renderer-specific differences.
  - Fix route and permalink differences before content-only differences.
  - Fix Liquid drops, tags, and filters before Markdown serialization differences.
  - Fix Kramdown/GFM output without broad post-render rewriting.

**Exit gate:** every P0 core fixture and every always-enabled plugin fixture
passes with zero unexplained differences.

## Phase 3 — Complete optional Pages plugins

- Finish partially emulated plugins
  - Complete `jekyll-seo-tag` metadata normalization.
  - Match `jekyll-redirect-from` templates and generated routes.
  - Match `jekyll-sitemap` timestamps and exclusion behavior.
  - Match `jemoji` fallback markup and replacement boundaries.
  - Add selectable `jekyll-commonmark-ghpages` rendering.
- Retain pinned GitHub remote-theme coverage
  - Verify layouts, includes, Sass, and assets retain normal site-over-theme precedence.
  - Keep archive fixtures and cache tests network-independent.
  - Document the intentionally unsupported branches, tags, omitted revisions,
    arbitrary URLs, theme configuration, theme data, dependencies, and
    `theme.*` variables.
  - Run isolated and interaction fixtures for avatar, feed, gist, include-cache, mentions, and pagination.
  - Verify plugin order using fixtures where post-render transformations overlap.
  - Verify disabling and configuration semantics for each optional plugin.

**Exit gate:** all 19 gems in the P0 plugin baseline pass their isolated
fixtures, and the full-bundle fixture passes with the Pages activation order.

## Phase 4 — Add current Jekyll 4 compatibility

- Establish the standalone 4.4.1 reference
  - Build the core fixture corpus with Jekyll 4.4.1 and no Pages gem.
  - Classify each mismatch as shared, Jekyll 3-only, or Jekyll 4-only.
  - Add a profile selector only where Jigyll must expose genuinely incompatible behavior.
- Implement Jekyll 4 behavior
  - Support theme `_config.yml`, theme `_data`, runtime plugin dependencies, and `theme.*` drops.
  - Support Jekyll 4 Liquid additions currently listed in [Differences from Jekyll](/docs/differences/).
  - Match current Sass, Markdown, syntax-highlighting, and URL behavior.
  - Preserve the P0 profile whenever Jekyll 4 behavior conflicts with Pages.
- Document profile boundaries
  - State which behavior Jigyll selects by default.
  - Document configuration needed for incompatible profile behavior.
  - Publish the remaining profile-specific differences from generated reports.

**Exit gate:** the standalone Jekyll 4.4.1 core corpus passes without regressing
the GitHub Pages P0 suite.

## Phase 5 — Add historical and ongoing coverage

- Select historical Pages profiles
  - Identify tagged Pages releases where the Jekyll minor version changed.
  - Add one immutable profile for each selected compatibility boundary.
  - Reuse fixtures and expected outputs instead of branching implementation code by patch release.
- Automate upstream monitoring
  - Check the Pages versions endpoint on a schedule.
  - Open a dependency-update change when the published manifest changes.
  - Run the corpus against a candidate profile before declaring support.
  - Retain prior profiles so compatibility claims remain reproducible.
- Publish support evidence
  - Generate a profile-by-feature compatibility table from comparison reports.
  - Link each partial or unsupported entry to its failing fixture.
  - Update the [Plugins](/docs/plugins/) page from the same status data.
  - Record newly supported profiles in the changelog.

**Exit gate:** a GitHub Pages dependency update is detected automatically, prior
profiles remain runnable, and every published compatibility claim links to a
passing comparison report.

## Release criteria

A profile is supported only when all of the following are true:

- Its complete dependency manifest and reference environment are immutable.
- Its required core, plugin, error, and interaction fixtures pass in CI.
- No comparison suppresses an unexplained file, route, or HTML subtree.
- Network-dependent behavior uses deterministic recorded inputs.
- The plugin table and [Differences from Jekyll](/docs/differences/) match the test evidence.
- A real representative site builds under Ruby Jekyll and Jigyll and produces equivalent routes and rendered pages.

[pages-versions]: https://pages.github.com/versions/
[pages-versions-json]: https://pages.github.com/versions.json
[pages-releases]: https://github.com/github/pages-gem/releases
