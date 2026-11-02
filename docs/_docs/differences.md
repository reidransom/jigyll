---
title: Differences from Jekyll
permalink: /docs/differences/
description: Where Jigyll's behavior diverges from Ruby Jekyll.
---

## Ruby Jekyll

Jigyll targets Jekyll compatibility, but it is a separate implementation and
some behavior differs. The categories below are generated from the same
compatibility metadata (`_data/compat.yml`) that powers the callout at the top
of each affected page and the badges in the sidebar — one source of truth,
three surfaces.

{% assign statuses = "replaced|modified|unsupported|jigyll-only" | split: "|" -%}
{% for status in statuses -%}
{% capture rows -%}
{% for item in site.docs -%}
{% assign compat_key = item.compat_key | default: item.slug -%}
{% assign compat = site.data.compat[compat_key] -%}
{% if compat.status == status -%}
- **[{{ item.title }}]({{ item.url | relative_url }})** — {{ compat.summary }}
{% endif -%}
{% endfor -%}
{% endcapture -%}
{% assign trimmed = rows | strip -%}
{% unless trimmed == "" %}
{% case status %}
{% when "replaced" %}### Replaced

These topics work fundamentally differently in Jigyll.
{% when "modified" %}### Modified

Mostly the same as Jekyll, with specific differences.
{% when "unsupported" %}### Not supported

Jekyll has these; Jigyll doesn't.
{% when "jigyll-only" %}### Jigyll-only

Jigyll additions with no Jekyll counterpart.
{% endcase %}
{{ trimmed }}
{% if status == "modified" %}

#### Sass asset pipeline

{% assign comparison = site.data.compat.assets.comparison %}
| Behavior | Jigyll | Ruby Jekyll |
| --- | --- | --- |
{% for row in comparison -%}
| {{ row.behavior }} | {{ row.jigyll }} | {{ row.jekyll }} |
{% endfor -%}
{% endif %}
{% endunless %}
{% endfor %}

## GitHub Pages

GitHub Pages adds a pinned Jekyll 3.10 runtime and managed plugin set. The Ruby
Jekyll differences above therefore also apply to Pages unless a Pages-specific
behavior is called out below.

| Status | Capability | Documented gap |
| --- | --- | --- |
| Missing | `jekyll-coffeescript` | CoffeeScript assets are not supported. |
| Missing | `jekyll-titles-from-headings` | Titles are not inferred from headings. |
| Partial | `jekyll-commonmark-ghpages` | The Pages CommonMark processor and its options are not selectable. |
| Partial | `jekyll-github-metadata` | Several repository fields and deterministic API fixtures remain incomplete. |
| Partial | `jekyll-redirect-from` | Default templates and redirect-route behavior need Pages verification. |
| Partial | `jekyll-remote-theme` | Only pinned GitHub `owner/repository@40-character-SHA` archives work. |
| Partial | `jekyll-seo-tag` | Publisher, author, image, social, and canonical metadata normalization is incomplete. |
| Partial | `jekyll-sitemap` | File modification dates and exclusion behavior need Pages verification. |
| Partial | `jemoji` | Image fallback markup and HTML-safe replacement behavior need Pages verification. |

See the [current plugin baseline](/docs/compatibility-roadmap/#current-plugin-baseline)
for all GitHub Pages dependencies, their versioned reference environment, and
the verification work required before a profile is supported.
