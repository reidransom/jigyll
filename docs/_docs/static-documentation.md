---
title: Static Documentation Exports
parent: Site Structure
nav_order: 13
permalink: /docs/static-documentation/
description: Generate public Markdown articles, discovery links, and section data from one Jigyll build.
---

Jigyll can export public documentation from the same render that produces its HTML. The exporter retains Liquid-resolved article source before Markdown conversion and reads actual rendered heading IDs. No Ruby plugin, separate Liquid renderer, or authored article copy is involved.

## Enable exports

Set the canonical site URL and enable documentation in `_config.yml`:

```yaml
url: https://example.com
documentation:
  sections: docs_sections
  description: Guides for configuring and operating Example websites.
  openapi: https://example.com/api/openapi.json
```

`sections` names a site-data list, defaulting to `_data/docs_sections.yml`. Each group has `slug`, `title`, and `description`. `description` supplies the project overview; `openapi` is optional.

An exported article uses `layout: docs`, a configured `nav_section`, a title, description, integer `nav_order`, and a clean `/docs/.../` directory permalink. An omitted `published` is public. Explicit drafts, `redirect_to` pages, hidden/planning paths, and unconfigured groups are excluded even with `--unpublished`. Article IDs come from the permalink, not the title or filename. Group order follows the data list; article order follows `nav_order`, then ID.

Full, non-localized builds with an empty `baseurl` are supported. Documentation-enabled incremental or localized builds fail explicitly rather than combining different article generations. Set `documentation.enabled: false` to disable exports.

## Generated routes and navigation

`jigyll build` and `jigyll serve` provide:

- `/llms.txt`: project overview, grouped Markdown article links, inventory/corpus downloads, and the optional authoritative OpenAPI link.
- `/docs/manifest.json`: versioned project, navigation, article, URL, and heading metadata.
- `/docs/<article-id>/index.md`: resolved article Markdown without front matter or layout chrome.
- `/docs/search.json`: source section text and canonical citations for local retrieval.

Generated routes cannot shadow authored output and do not become additional source pages. Output files cannot collide with another output directory or its descendants. These checks run before destination cleaning, which removes obsolete exports; watch reloads use a complete new inventory. Invalid article metadata or ambiguous/missing heading targets also fail generation before the destination is cleaned.

Templates use `site.documentation.groups` and `site.documentation.articles` for the same selected navigation inventory. Each template article includes canonical `markdown_url` and same-origin `markdown_path`; use the latter for visible/alternate links that must work on preview hosts. Layouts can render `page.headings` into a TOC without client-side heading construction. Final article HTML must wrap the body in exactly one `<article class="docs-article">` for post-processing finalization.

Markdown keeps code blocks, tables, and literal examples. Real navigation/media destinations are resolved against the original canonical HTML article, including fragment-only links. Section records are disjoint: introduction text precedes the first heading, and each heading's body ends at the next heading of any level. Heading hierarchy supplies context without duplicating parent text. No-heading articles have an introductory record when nonempty. Raw headings require unique explicit IDs.

## Snapshot contract

Schema version 1 uses `project`, `groups`, and `articles` in the manifest and `records` in the corpus. Article records include `id`, `title`, `description`, `nav_section`, `nav_order`, site-relative `url`, `canonical_url`, `markdown_url`, and `sections`. Search records include article identity, heading/hierarchy, exact `fragment`, source `text`, and both URLs. Introduction records have an empty fragment/hierarchy and the article ID; headed record IDs append `#<actual-fragment>`.

The same `snapshot_id` appears in the manifest, corpus, and discovery document. It is SHA-256 over compact Go JSON for the manifest and corpus with empty snapshot IDs, followed by article Markdown bytes in manifest order. Each input is prefixed with its unsigned 64-bit big-endian byte length. Project overview metadata participates. Arrays serialize as arrays, not null. Build timestamps, invented review dates, and layout chrome do not participate.

## Finalize after asset processing

Asset processors can change HTML and JSON after Jigyll builds. cdnware 0.1.3 does not rewrite Markdown files. Run native finalization with the actual asset map after processing and before making the output live:

```sh
jigyll build
cdnware -cdn https://cdn.example.com/site _site > _asset-rev.json
jigyll documentation-finalize --asset-map _asset-rev.json _site
```

The finalizer does not load source or make network requests. It confines reads to the output root, rebases actual Markdown destinations through the revision map, refreshes sections from final article HTML, and recomputes identity. Missing, ambiguous, or unsafe inputs fail explicitly. It validates and stages the complete set before replacement; individual replacements are atomic, but the set is not a filesystem transaction. Publish only after success, preferably by activating a complete immutable directory.

Dash's native build owner runs this stage after cdnware and before uploads/publication. Release review must include the docs-enabled Jigyll binary in the application image; an older binary is not an export-capable fallback. This feature does not install a hosted search service or MCP endpoint.
