# Release Notes
<!-- markdownlint-disable MD024 -->

## Unreleased

### New
* Add `just perf-setup` and `just perf` for pinned native Jigyll/Jekyll full-build
  comparisons, with a deterministic generated workload, locked local bundles,
  diagnostic concurrency/cache profiles, semantic output preflight, isolated
  destinations/caches, raw samples and reproducibility reports.

### Improved
* Rewrite the documentation landing page for developers: describe source formats,
  CLI behavior, dependencies, and compatibility limits instead of promotional
  slogans, with neutral framing for the existing benchmark results.
* Make documentation inline code more distinct with theme-aware accent text,
  tinted backgrounds, and subtle rounded borders without changing fenced blocks.
* Document benchmark installation, real Sass-site preparation, measurement
  boundaries and artifact interpretation. Attribute inherited benchmark tables
  to historical upstream Gojekyll rather than current Jigyll measurements.
* Present the documentation homepage as a responsive Docifly hero splash with
  paper-dragon artwork, installation/docs actions, compatibility guidance, and
  measured local Jigyll/Jekyll build-time visualizations. Keep zero-baseline
  comparison bars and measurement caveats; place the full diagnostic table in
  the linked benchmark documentation.

## 1.15.0 (Nov 6, 2026)

### New
* Install themes with `jigyll new PATH --theme owner/repository@latest`,
  exact tags, or full commit SHAs without Git. GitHub shorthand and explicit
  Forgejo instance URLs resolve source archives into local `_theme/` files,
  with transactional destination safety and recorded commit/archive-digest
  provenance. Existing Git URL installation and pinned GitHub `remote_theme`
  behavior remain separate.
* Add `--follow-directory-symlinks` for explicitly publishing linked source
  directories, including linked collections and localized content. Watch mode
  tracks external edits and mount changes; directory links are rejected by
  default.
* Add opt-in `jigyll-llms-txt` for builds and preview: automatic public Markdown
  discovery or ordered curated sections, external and HTML entries, canonical
  base-path URLs, and optional Markdown companions. Settings support
  title/description overrides and `markdown` (default true).
* Support standalone localized discovery with one project aggregate index,
  ordered locale/section H2 groups, actual locale-owned companions, and literal
  route curation for differing translated slugs. Honor optional/required edition
  policy without fallback article content, root/subdirectory default locales,
  deployment base paths, and Markdown-disabled output. Localized documentation
  snapshots remain explicitly unsupported.

### Improved
* Add install-time GitHub and Forgejo repository-reference resolution for latest
  published releases, exact tags, and full commit SHAs. Resolve tags to immutable
  archive sources with bounded HTTP requests, separately from pinned remote themes.
* Share bounded theme archive downloads and extraction with streaming SHA-256
  observation. Reject incomplete gzip/tar streams, duplicate files, and `.git`
  metadata; failures clean only newly created working paths. Pinned GitHub
  remote themes retain their existing codeload-only source and cache policy.
* Migrate the documentation site from Just the Docs to Docifly, preserving all
  35 public pages, authored content, six-section navigation, homepage actions,
  and compatibility notices, indicators, and legend. Include documentation
  articles in search while retaining search exclusions and heading data.
  Preserve repository, attribution, source-edit, and back-to-top links alongside
  Docifly's responsive menu, table of contents, heading links, code copying, and
  light/dark controls. Remove the obsolete theme settings, navigation metadata,
  Sass, and search-focus workaround; local builds use the linked sibling theme.
* Scope Liquid-resolved source capture to selected export pages while preserving
  documentation capture without the discovery plugin.
* Refresh complete discovery generations in watch mode, including plugin-enabled
  incremental builds. Content, includes, data, curation, permalinks, publication,
  and configuration changes refresh index/companion bytes and remove obsolete
  routes and output files. Preserve authored output and `keep_files`.
* Prepare fresh preview generations before publication; failed reads, renders,
  conversions, and collisions retain the last good index/companions without a
  successful refresh notification. Keep configured canonical origin/base-path
  destinations across serving overrides and subsequent refreshes.
* Validate localized aggregate/companion routes, aliases, and physical ancestor
  destinations before staged promotion. Preserve canonical exports through
  preview startup/reloads, publish actual staged files with relative CLI source
  paths, and keep destination-specific staging/backup events out of source
  watches without hiding unrelated authored includes.
* Document the complete standalone/localized discovery contract, publication
  exclusions, literal translated-slug curation, canonical preview paths, and
  the documentation-only asset finalizer boundary, with a local revision-map
  example requiring no CDN or external theme.

### Fixed
* Restore configured static-analysis checks for theme installation, `llms.txt`
  discovery, and symlink-aware build/watch paths without relaxing lint rules.

### Breaking
* Documentation configuration no longer generates or reserves `/llms.txt`.
  Add `jigyll-llms-txt` to `plugins` to retain discovery. Combined builds reuse
  documentation groups and article Markdown unless explicit plugin sections
  replace selection; documentation-only builds retain manifest/corpus/Markdown.
* Discovery has no snapshot ID or timestamp. Manifest/corpus schema and hashing
  remain unchanged. `documentation-finalize` updates only documentation-owned
  artifacts, preserving authored/plugin indexes and general plugin companions;
  it does not rebase general companions after asset processing. Documentation's
  localization, base-path, and incremental restrictions remain unchanged.

## 1.14.0 (Oct 29, 2026)

### New
* Added a shared article API for Liquid-resolved documentation source, rendered
  body HTML, and heading metadata, without repeating Liquid evaluation.
* Added an opt-in public documentation inventory shared by templates and
  versioned export records, preserving published-by-omission behavior.
* Added source-preserving documentation Markdown destination rebasing with
  protected code examples and support for publish-time asset maps.
* Added disjoint rendered documentation sections with exact fragment validation,
  introductory text, heading hierarchy, and code/table-aware text extraction.
* Added native build/serve documentation artifacts with content-derived snapshots,
  generated Markdown routes, stale-export cleanup, and an offline
  `documentation-finalize` command for post-asset-processing publication.
* Exposed inventory-mapped same-origin Markdown paths for preview-safe article
  format and discovery links while retaining canonical citation metadata.

### Fixed
* Preserve Markdown link titles, escaped destinations, container-nested references,
  footnote prose, and raw-element bodies while revising actual URLs. Export parsing
  now shares the article renderer's syntax extensions.
* Reject documentation output file/directory collisions before cleaning completed
  output, including collisions with generated manifests and Markdown files.

## 1.13.1 (Sep 14, 2026)

### Fixed
* Localized builds now honor `include` settings for underscore-prefixed page
  directories while validating required translations.

## 1.13.0 (Sep 12, 2026)

### New
* Added fenced-code metadata for semantic editor and terminal frames, escaped
  titles, server-rendered line and text markers, and CSS-only line-number
  gutters with validated starting values.
* Added stable renderer seams for code-frame captions, logical source lines,
  marker regions, and decimal line-number presentation without changing copied
  source text or unannotated fenced-code output.


## 1.12.0 (Sep 9, 2026)

### New
* Added `page.headings` for layout-visible article heading levels, emitted IDs,
  and plain-text labels. Metadata follows rendered content, includes nested
  Markdown, and refreshes on page reloads and content replacement without
  changing existing anchors or `{:toc}` output.

## 0.5.2 (Nov 30, 2024)

### Improved
* Architecture fixes for local build
* Handle common Jekyll date formats

## 0.4.0 (Nov 13, 2024)

### New
* Added name attribute to page drops
* Added include tag with variable support

### Improved
* Upgraded liquid dependency

## 0.3 (unreleased)

### Contributions

* [@danog (Daniil Gentili)](https://github.com/danog): Add native chroma
  highlighting (~30x performance increase) [PR
  #42](https://github.com/osteele/gojekyll/pull/42)

* [@mevdschee (Maurits van der Schee)](https://github.com/mevdschee): add
  support for go mod and blackfriday v2 [PR
  #39](https://github.com/osteele/gojekyll/pull/39)

* [@bep (Bjørn Erik Pedersen)](https://github.com/bep): Update Hugo matrix in
  README [PR #38](https://github.com/osteele/gojekyll/pull/38)

### Other changes

* Upgraded to liquid v1.3.0. See the liquid release notes
  [here](https://github.com/osteele/liquid/blob/main/CHANGELOG.md#130-2020-02-13).

* The binary is built with go v1.17. (The previous release, v0.2.5, was built
  with go v1.08.)

## 0.2.5 (Aug 18, 2017)

### New
* Added support for `jekyll.environment` variable from `JEKYLL_ENV`
* Added script to compare jekyll and gojekyll builds

### Improved
* Renamed internal pipeline to renderers for better code organization
* Updated documentation about auto-generated ID compatibility

## 0.2.4 (Aug 10, 2017)

### New
* Added support for page excerpts
* Added support for rendering non-collection pages

### Improved
* Implemented `page.previous` and `page.next` navigation
* Changed Page content handling from []byte to string
* Reorganized rendering order - posts collection now renders last
* Improved Travis CI configuration to skip tests and lint on tags
* Moved example directory and updated documentation

## 0.2.3 (Aug 8, 2017)

### New
* Added support for dots in destination directory paths
* Added minification for SEO tags

### Improved
* Improved reload functionality
* Enhanced variables subcommand to handle byte-to-string conversion

## 0.2.2 (Aug 3, 2017)

### Improved
* Improved file change detection accuracy
* Enhanced file exclusion logic

### Fixed
* Fixed file descriptor leak
* Fixed race condition in file handling

## 0.2.1 (Jul 26, 2017)

### New
* Added CSS minification using minify instead of cssmin
* Added Gemfile to example site for side-by-side comparison

### Improved
* Improved livereloader output clarity
* Enhanced GitHub metadata plugin with environment variables support
* Better error reporting for failed plugins
* Implemented HTML minimization for feeds and sitemaps

## 0.2.0 (Jul 25, 2017)

### Major Changes
* Implemented incremental build support
* Created PageEmbed functionality
* Major refactoring for improved performance and maintainability

## 0.1.1 (Jul 19, 2017)

### Improved
* Reorganized command structure
* Updated build and deployment configuration
* Improved package organization

## 0.1.0 (Jul 17, 2017)

### Major Changes
* Initial stable release
* Complete implementation of core Jekyll functionality

## 0.0.5 (Jul 13, 2017)

### New
* Added mutex for cache operations

### Improved
* Moved commands to cmd package
* Updated package organization

### Fixed
* Fixed race conditions

## 0.0.4 (Jul 13, 2017)

### New
* Implemented support for data files (CSV and JSON)
* Added server watch functionality
* Added support for blackfriday auto header IDs

### Improved
* Improved error display in page

## 0.0.3 (Jul 12, 2017)

### Improved
* Reorganized main.go to repository root
* Updated goreleaser configuration

### Fixed
* Fixed static file serving

## 0.0.2 (Jul 12, 2017)

### New
* Initial setup of goreleaser

### Improved
* Configuration updates for Travis CI
* Basic build system setup

## 0.0.1 (Jul 12, 2017)

### New
* Initial release
* Basic Jekyll functionality implemented

### Improved
* Setup of Travis CI and basic tooling
