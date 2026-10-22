---
title: Pages
parent: Content
nav_order: 1
permalink: /docs/pages/
description: Standalone pages — the most basic building block for content.
---

Pages are the most basic building block for content. They're useful for
standalone content (content which is not date based or is not a group of
content such as staff members or recipes).

The simplest way of adding a page is to add an HTML file in the root
directory with a suitable filename. You can also write a page in Markdown
using a `.md` extension and [front matter](/docs/front-matter/), which
converts to HTML on build. For a site with a homepage, an about page, and a
contact page, here's what the root directory and associated URLs might look
like:

```
.
├── about.md      # => http://example.com/about.html
├── index.html    # => http://example.com/
└── contact.html  # => http://example.com/contact.html
```

If you have a lot of pages, you can organize them into subfolders. The same
subfolders that are used to group your pages in your project's source will
then exist in the `_site` folder when your site builds:

```
.
├── about.md          # => http://example.com/about.html
├── documentation     # folder containing pages
│   └── doc1.md       # => http://example.com/documentation/doc1.html
├── design            # folder containing pages
│   └── draft.md      # => http://example.com/design/draft.html
```

Remember that a page is only processed if it begins with front matter — a
Markdown file without one is copied through as a
[static file](/docs/static-files/), unless the
[jekyll-optional-front-matter plugin](/docs/plugins/) is enabled.

## Changing the output URL

You might want to have a particular folder structure for your source files
that changes for the built site. With [permalinks](/docs/permalinks/) you
have full control of the output URL.

## Excerpts for pages

Pages have an `excerpt` variable just like posts — by default the first
paragraph of content. (In Jekyll this requires setting `page_excerpts: true`;
in Jigyll it is always available.)

## Article rendering API

Go integrations can call `Page.Article()` to render a page and obtain its body
HTML and heading metadata before layouts are applied. The returned `Article`
also contains the exact Liquid-resolved, pre-conversion source when the page
uses `layout: docs` and the `documentation` configuration map is nonempty
and not disabled with `enabled: false`.

Liquid runs once for the page body; Markdown or other extension-selected
conversion consumes that same resolved source. `Article.Source` is `nil` when
capture is disabled or content was replaced with `SetContent`; HTML is never
substituted for authored source. Reloads and clones start with fresh capture
state. Source and heading slices are shared immutable page data and must not
be modified by callers.

Renderer integrations can continue using `Render` for the full pipeline, or
call `RenderTemplate` followed by `Convert` to separate Liquid evaluation from
extension-selected conversion without evaluating a template twice.
