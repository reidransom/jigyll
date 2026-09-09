---
title: Variables
parent: Site Structure
nav_order: 3
permalink: /docs/variables/
description: The site, page, layout, and jekyll variables Jigyll exposes to Liquid.
---

Jigyll traverses your site looking for files to process. Any files with
[front matter](/docs/front-matter/) are subject to processing. For each of
these files, Jigyll makes a variety of data available via
[Liquid](/docs/liquid/). The following is a reference of the available data.

## Global Variables

| Variable | Description |
| --- | --- |
| `site` | Site-wide information + configuration settings from `_config.yml`. See below for details. |
| `page` | Page-specific information + the [front matter](/docs/front-matter/). Custom variables set via the front matter will be available here. See below for details. |
| `layout` | Layout-specific information + the front matter. Custom variables set via front matter in [layouts](/docs/layouts/) will be available here. |
| `jekyll` | Version and environment information. See below for details. |
| `content` | In layout files, the rendered content of the post or page being wrapped. Not defined in post or page files. |
| `paginator` | The [pagination](/docs/pagination/) state, on paginated index pages. |

> **Differs from Jekyll.** The `theme` variable (gemspec metadata of the
> active theme gem, Jekyll 4.3+) does not exist.

## Site Variables

| Variable | Description |
| --- | --- |
| `site.time` | The current time (when you ran the `jigyll` command). |
| `site.pages` | A list of all pages. |
| `site.posts` | A reverse-chronological list of all posts. |
| `site.related_posts` | The ten most recent posts. (Jekyll's `--lsi` similarity ranking is not supported.) |
| `site.static_files` | A list of all [static files](/docs/static-files/). |
| `site.html_pages` | A subset of `site.pages` listing those which end in `.html`. |
| `site.html_files` | A subset of `site.static_files` listing those which end in `.html`. |
| `site.collections` | A list of all the [collections](/docs/collections/) (including posts). |
| `site.data` | The data loaded from the files in the `_data` directory. |
| `site.documents` | A list of all the documents in every collection. |
| `site.categories.CATEGORY` | The list of all posts in category `CATEGORY`. |
| `site.tags.TAG` | The list of all posts with tag `TAG`. |
| `site.url` | The url of your site as configured in `_config.yml` (or the `JEKYLL_URL` environment variable). |
| `site.[CONFIGURATION_DATA]` | All the variables set in your `_config.yml` are available through the `site` variable. For example, if you have `foo: bar` in your configuration file, then it will be accessible in Liquid as `site.foo`. |

## Page Variables

| Variable | Description |
| --- | --- |
| `page.content` | The content of the page. |
| `page.title` | The title of the page, from its front matter. |
| `page.excerpt` | The un-rendered excerpt of a page. Can be overridden with an `excerpt` key in the front matter. |
| `page.headings` | Rendered article headings in document order. Each entry has `level`, `id`, and `text`. Available in layouts and layout includes; see below. Cannot be overridden by front matter. |
| `page.url` | The URL of the page without the domain, but with a leading slash, e.g. `/2026/12/14/my-post.html` |
| `page.date` | The date assigned to a post. Can be overridden in a post's front matter with a `date` key. |
| `page.id` | An identifier unique to a document in a collection or a post (useful in RSS feeds). |
| `page.categories` | The list of categories to which this post belongs, from its front matter. |
| `page.collection` | The label of the collection this document belongs to — `posts` for a post. Not set for regular pages. |
| `page.tags` | The list of tags to which this post belongs, from its front matter. |
| `page.name` | The filename of the post or page, e.g. `about.md` |
| `page.path` | The path to the raw post or page, relative to the source directory. |
| `page.relative_path` | Same as `page.path`, relative to the source directory. |
| `page.slug` | The filename of the document without its extension (or date prefix, for a post). |
| `page.ext` | The file extension of the source document, e.g. `.md` |
| `page.next` | The next post relative to the position of the current post in `site.posts`. Defined for posts only. |
| `page.previous` | The previous post relative to the position of the current post in `site.posts`. Defined for posts only. |

> **Differs from Jekyll.** `page.dir` does not exist — derive the directory
> from `page.url` or `page.path` if you need it. Categories always come from
> front matter, [never from the directory path](/docs/posts/). Custom front
> matter is available under `page`, except for engine-owned fields such as
> `headings`.

### Article headings

`page.headings` contains the headings from the rendered article, before any
layout or post-render plugin runs. Liquid includes and supported nested Markdown
contribute headings. Layout chrome, code samples, and HTML templates do not.
No `{:toc}` marker is required.

| Field | Meaning |
| --- | --- |
| `heading.level` | Numeric HTML heading level, 1–6. The list is flat and follows article order. |
| `heading.id` | The emitted heading ID, unchanged. Raw HTML headings without IDs have an empty string. |
| `heading.text` | Decoded descendant text with inline markup removed and surrounding whitespace trimmed. Internal whitespace is retained. |

Render h2/h3 navigation in a layout or layout include:

{% raw %}
```liquid
<nav aria-label="On this page">
  {% for heading in page.headings %}
    {% if heading.id != '' %}
      {% if heading.level == 2 or heading.level == 3 %}
        <a href="#{{ heading.id | escape }}">{{ heading.text | escape }}</a>
      {% endif %}
    {% endif %}
  {% endfor %}
</nav>
```
{% endraw %}

Skip ID-less headings when creating links, and escape both the ID and label.
Jigyll does not generate new IDs while collecting metadata. Existing duplicate
IDs remain duplicated, including collisions between an outer heading and a
heading inside `markdown="1"`. Such links cannot distinguish the duplicate
targets.

The list includes `.no_toc` headings and all heading levels, independently of
`kramdown.toc_levels`. Those settings continue to control only the content-level
`{:toc}` list. Front matter and front-matter defaults cannot replace
`page.headings`.

The current page exposes an empty list during its content Liquid rendering.
Its completed list is available in layouts, including parent layouts and their
includes. Reading the field never triggers another render. Empty articles and
non-HTML output pages also expose an empty list; Jigyll treats `.html`, `.htm`,
and `.xhtml` output as HTML.

Accessing another page's `headings`, for example through `site.pages`, returns
its last completed render's list, or an empty list if it has not rendered or
has been invalidated. Full builds render page content before applying layouts.
Do not rely on fresh cross-page metadata during content rendering or incremental
builds, where pages can be rebuilt individually.

Reloading a page clears its headings. Replacing content through `SetContent`
recomputes them, and cloned pages compute their own lists. Post-render plugins
run after layouts, so their later changes are not reflected in these labels.

## Jekyll Variables

| Variable | Description |
| --- | --- |
| `jekyll.version` | The Jigyll version used to build the site, suffixed with `(jigyll)` — e.g. `1.2.3 (jigyll)`. It is **not** a Ruby Jekyll version number. |
| `jekyll.environment` | The value of the `JEKYLL_ENV` environment variable during the build, defaulting to `development`. |
