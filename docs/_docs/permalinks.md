---
title: Permalinks
permalink: /docs/permalinks/
description: Controlling output URLs — placeholders, built-in styles, and where Jigyll's rules differ.
---

Permalinks are the output path for your pages, posts, or collections. They
allow you to structure the directories of your source code differently from
the directories in your output.

## Front Matter

The simplest way to set a permalink is using front matter. You set the
`permalink` variable in front matter to the output path you'd like.

For example, you might have a page on your site located at
`/my_pages/about-me.html` and you want the output url to be `/about/`. In
front matter of the page you would set:

```yaml
---
permalink: /about/
---
```

An explicit front-matter permalink is honored verbatim and overrides
everything else. (Unlike Jekyll, Jigyll also recognizes the built-in style
names below when used in front matter.)

## Global

To set a permalink structure globally, use the `permalink` key in
`_config.yml` with placeholders for your desired output. For example:

```yaml
permalink: /:categories/:year/:month/:day/:title:output_ext
```

> **Differs from Jekyll.** Global styles and custom patterns apply to
> **posts only**, except `permalink: pretty`: it also gives ordinary HTML
> pages without an explicit permalink directory URLs. Other collections default
> to `/:collection/:path:output_ext` — override them per collection, as shown
> below. In Jekyll, the global style also reshapes page and collection URLs.

### Placeholders

The placeholders Jigyll implements:

| Variable | Description |
| --- | --- |
| `:year` | Year from the post's filename, four digits. May be overridden via the document's `date` front matter. |
| `:short_year` | Year without the century (00..99). |
| `:month` | Month from the post's filename (01..12). |
| `:i_month` | Month without leading zeros. |
| `:day` | Day of the month from the post's filename (01..31). |
| `:i_day` | Day of the month without leading zeros. |
| `:y_day` | Ordinal day of the year, with leading zeros (001..366). |
| `:hour`, `:minute`, `:second` | Time of day from the post's `date` front matter, zero-padded. |
| `:title` | Slug from the document's filename (case preserved, as in Jekyll). May be overridden via the document's `slug` front matter. |
| `:slug` | Lowercased slug from the document's filename. May be overridden via the document's `slug` front matter. |
| `:name` | The document's slugified base filename, lowercased. |
| `:categories` | The post's categories, joined with `/`. Omitted when empty. |
| `:collection` | Label of the containing collection. |
| `:path` | Path to the document relative to the source (or collection) directory, without the file extension. |
| `:output_ext` | Extension of the output file. (Included by default and usually unnecessary.) |

Any *string* front-matter key of the document can also be used as a
placeholder — `/:collection/:color/:path` works if documents set `color:`
in their front matter. That's a Jigyll extension.

> **Differs from Jekyll.** An unknown placeholder is a **build error**
> (Jekyll leaves it in the URL). Not implemented: `:short_month`,
> `:long_month`, `:week`, `:w_year`, `:w_day`, `:short_day`, `:long_day`,
> and `:slugified_categories`.

### Built-in formats

For posts, Jigyll provides the following built-in styles for convenience:

| Permalink Style | URL Template |
| --- | --- |
| `date` | `/:categories/:year/:month/:day/:title.html` |
| `pretty` | `/:categories/:year/:month/:day/:title/` |
| `ordinal` | `/:categories/:year/:y_day/:title.html` |
| `none` | `/:categories/:title.html` |

Rather than typing `permalink: /:categories/:year/:month/:day/:title/`, you
can just type `permalink: pretty`. The default style is `date`. Jekyll's
`weekdate` style is not supported.

For ordinary HTML pages without an explicit permalink, the global `pretty`
style uses a directory URL: `docs/about.md` becomes `/docs/about/` and writes
to `docs/about/index.html`. HTML-like source pages retain their output
extension for that index file; index pages retain their containing-directory
URLs. An explicit page permalink, including one supplied by front-matter
defaults, always wins. Other global styles retain the normal
`/:path:output_ext` page URL.

### Collections

For collections (including `posts`), you can override the permalink pattern
in the collection configuration in `_config.yml`:

```yaml
collections:
  my_collection:
    output: true
    permalink: /:collection/:name
```

### index.html collapsing

As in Jekyll, an HTML page that would be output as `.../index.html` gets the
containing directory as its URL — `about/index.md` yields the URL
`/about/`, not `/about/index.html`. The output file is still written as
`index.html`. (An explicit front-matter `permalink` is never rewritten.)
