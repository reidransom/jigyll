---
title: Plugins
parent: Guides
nav_order: 1
permalink: /docs/plugins/
description: Enable compiled-in Jekyll-compatible plugins and Jigyll's opt-in llms.txt generator.
---

Jekyll has an extensible Ruby plugin system — generators, converters,
commands, tags, filters, and hooks. **Jigyll does not.** Plugins are Go code
compiled into the binary: a fixed set of Jekyll plugins is emulated, alongside
Jigyll-native features such as `jigyll-llms-txt`.¹

Enable them in `_config.yml` (never a Gemfile):

```yaml
plugins:
  - jekyll-feed
  - jekyll-seo-tag
  - jekyll-sitemap
```

The legacy `gems:` key works as an alias. Naming a plugin Jigyll doesn't know
prints a warning. To see the plugin names your installed binary recognizes,
run `jigyll plugins` — note it lists *recognized* names; the table below is
the source of truth for how completely each is implemented.

## Emulated plugins

| Plugin | Motivation | Status | Missing features |
| --- | --- | --- | --- |
| [jekyll-avatar][jekyll-avatar] | GitHub Pages² | ✓ | |
| [jekyll-coffeescript][jekyll-coffeescript] | GitHub Pages | ✗ | |
| [jekyll-default-layout][jekyll-default-layout] | GitHub Pages | ✓ | |
| [jekyll-feed][jekyll-feed] | GitHub Pages | ✓ | |
| [jekyll-gist][jekyll-gist] | core³ | ✓ | |
| [jekyll-github-metadata][jekyll-github-metadata] | GitHub Pages | partial | `public_repositories`, `show_downloads`, `releases`, `versions`, `wiki_url`; Octokit configuration; GitHub Enterprise |
| [jekyll-live-reload][jekyll-live-reload] | core | ✓ | always enabled (by design); no way to disable |
| [jekyll-mentions][jekyll-mentions] | GitHub Pages | ✓ | |
| [jekyll-optional-front-matter][jekyll-optional-front-matter] | GitHub Pages | ✓ | |
| [jekyll-paginate][jekyll-paginate] | core | ✓ | jekyll-paginate-v2 features; page URLs are directory-normalized (`/page2/`) — see [Pagination](/docs/pagination/) |
| [jekyll-readme-index][jekyll-readme-index] | GitHub Pages | ✓ | |
| [jekyll-redirect-from][jekyll-redirect-from] | GitHub Pages | ✓ | user template |
| [jekyll-relative-links][jekyll-relative-links] | GitHub Pages | ✓ | |
| [jekyll-sass-converter][jekyll-sass-converter] | core | ✓ | always enabled (by design); no way to disable; only `sass_dir` is configurable |
| [jekyll-seo-tag][jekyll-seo-tag] | GitHub Pages | partial | `publisher`, `mainEntityOfPage`, and full author, image, and social normalization |
| [jekyll-sitemap][jekyll-sitemap] | GitHub Pages | ✓ | file modified dates⁴ |
| [jekyll-titles-from-headings][jekyll-titles-from-headings] | GitHub Pages | ✗ | |
| [jemoji][jemoji] | GitHub Pages | ✓ | image tag fallback |
| [github-pages][github-pages] | GitHub Pages | ✓ | enables the plugins github-pages includes, each in the state listed above |

¹ The internal APIs are too immature for a stable plugin interface, and Go's
[native plugin mechanism](https://golang.org/pkg/plugin/) only works on
Linux.

² Listed in the [GitHub Pages dependency
versions](https://pages.github.com/versions/).

³ "Core" plugins are referenced in the main [Jekyll
documentation](https://jekyllrb.com/docs/).

⁴ Modified dates aren't that useful with source control and CI. (Post dates
are included.)

## Jigyll-native discovery: `jigyll-llms-txt`

Enable theme-independent discovery explicitly in `_config.yml`:

```yaml
url: https://example.com
baseurl: /project
title: Example
description: Guides for configuring and operating Example.
plugins:
  - jigyll-llms-txt
llms_txt:
  markdown: true
```

Plugin membership is the only enable switch; `llms_txt` is optional and has
no effect without the plugin. `llms_txt.title` and `llms_txt.description`
override site metadata. The effective title must be nonempty, and `url`
must be an absolute HTTP(S) canonical URL; the description is optional.
`markdown` defaults to `true`. The index is deployed at `/project/llms.txt`
in this example, and its absolute destinations use the canonical origin and
`baseurl`, not the preview server's host.

### Automatic selection and ordered sections

Without `sections`, the plugin lists public source-backed Markdown pages,
posts, and output-enabled collection documents producing HTML in one `Pages`
section, sorted by canonical route. Each entry needs a title from front matter
or defaults; descriptions are optional and are not generated.

Set `llms: false` in front matter or defaults to exclude content. Drafts,
future and unpublished material remain excluded even under preview flags.
Redirects, 404s, generated pagination clones, static assets, non-output
collections, generated exports, hidden/planning sources, and explicit
`robots: noindex` are also excluded. Discovery is not access control and does
not implement general robots.txt semantics.

Explicit sections replace automatic selection and preserve section and entry order:

```yaml
llms_txt:
  title: Example developer guides
  description: Start here to configure Example.
  markdown: true
  sections:
    - title: Guides
      entries:
        - url: /getting-started/
        - url: /configuration/
          description: All supported configuration settings.
    - title: Optional
      entries:
        - url: https://reference.example.org/reference.md
          title: External reference
          description: Supporting protocol documentation.
```

Local URLs name actual canonical routes **before** `baseurl`, not source
filenames. They inherit page titles/descriptions unless overridden. Absolute
URLs on the canonical origin under the deployed base path are normalized to
local lookups, including publication exclusions. Other external entries need
an explicit title and absolute HTTP(S) URL; their destinations are kept as
authored and are never fetched. Explicit HTML-source pages link to canonical
HTML rather than being converted to Markdown.

Missing/excluded local routes, duplicate entries, invalid metadata or schemes,
and output collisions fail before destination cleaning. An empty eligible
inventory may produce just the title and optional summary. Selection does
not scrape navigation or require a theme, `layout: docs`, or `/docs/` routes.

### Markdown companions and ownership

Selected Markdown pages export Liquid-resolved source without front matter
or layouts. Actual links/media are rebased while code and literal examples
are preserved. The index links only to registered companions:

| HTML route | Markdown route |
| --- | --- |
| `/` | `/index.md` |
| `/guide/` | `/guide/index.md` |
| `/guide.html` | `/guide.html.md` |
| `/guide` | `/guide.md` |

These routes are relative to the deployed base path. Authored routes, aliases,
files, and directories cannot be overwritten. `markdown: false` emits only
the index and uses canonical HTML for local entries; it does not disable
independently enabled documentation Markdown.

With [documentation exports](/docs/static-documentation/) active and no
explicit sections, the plugin uses the documentation inventory and group order
instead of all pages. Metadata precedence is explicit `llms_txt`, then
documentation metadata, then site metadata. It reuses documentation-owned
Markdown artifacts; explicitly selected general pages can have separate plugin
companions without joining the manifest/corpus. Only active documentation
integration adds actual manifest/corpus downloads and configured authoritative
OpenAPI links.

The index has no snapshot ID or timestamp. `documentation-finalize` never
updates it or general plugin-owned companions; post-processing rebasing by
that command applies only to documentation-owned artifacts.

### Builds and preview refreshes

Non-localized builds and preview support standalone sites with a nonempty
`baseurl`. With the plugin enabled, `--incremental` still refreshes the complete
generation: content, includes, data, curation, publication state, permalinks,
collection output, and configuration all feed a fresh inventory.

Preview prepares and validates the entire index and selected companions before
publishing the replacement. If reading, rendering, conversion, or collision
validation fails, it keeps serving the previous index and companion bytes and
does not send a successful refresh notification. Canonical destinations retain
the configured origin and base path rather than adopting the serving host.

Successful disk rebuilds remove obsolete exports, including after selection
changes or plugin disablement, using the normal authored-output and `keep_files`
rules. Export validation runs before destination cleaning; ordinary disk builds
are not filesystem-wide transactions.

Localized discovery is not yet supported. Documentation integration retains its
own empty-baseurl, non-localized, non-incremental restrictions.


[jekyll-avatar]: https://github.com/benbalter/jekyll-avatar
[jekyll-coffeescript]: https://github.com/jekyll/jekyll-coffeescript
[jekyll-default-layout]: https://github.com/benbalter/jekyll-default-layout
[jekyll-feed]: https://github.com/jekyll/jekyll-feed
[jekyll-gist]: https://github.com/jekyll/jekyll-gist
[jekyll-github-metadata]: https://github.com/parkr/github-metadata
[jekyll-live-reload]: https://github.com/RobertDeRose/jekyll-livereload
[jekyll-mentions]: https://github.com/jekyll/jekyll-mentions
[jekyll-optional-front-matter]: https://github.com/benbalter/jekyll-optional-front-matter
[jekyll-paginate]: https://github.com/jekyll/jekyll-paginate
[jekyll-readme-index]: https://github.com/benbalter/jekyll-readme-index
[jekyll-redirect-from]: https://github.com/jekyll/jekyll-redirect-from
[jekyll-relative-links]: https://github.com/benbalter/jekyll-relative-links
[jekyll-sass-converter]: https://github.com/jekyll/jekyll-sass-converter
[jekyll-seo-tag]: https://github.com/jekyll/jekyll-seo-tag
[jekyll-sitemap]: https://github.com/jekyll/jekyll-sitemap
[jekyll-titles-from-headings]: https://github.com/benbalter/jekyll-titles-from-headings
[jemoji]: https://github.com/jekyll/jemoji
[github-pages]: https://github.com/github/pages-gem
