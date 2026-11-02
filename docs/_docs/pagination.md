---
title: Pagination
permalink: /docs/pagination/
description: Break a post listing into multiple pages with jekyll-paginate.
---

Jigyll implements the [`jekyll-paginate`
plugin](https://jekyllrb.com/docs/pagination/) (v1 — the one Jekyll's own
docs describe and GitHub Pages ships). It splits the site's posts across
repeated renders of an index page, exposing a `paginator` Liquid object to
each copy.

## Enable pagination

```yaml
plugins:
  - jekyll-paginate
paginate: 5
```

`paginate` sets the number of posts per page. `paginate_path` controls where
the extra pages go and defaults to `/page:num` — page 2 of a five-per-page
blog is written to `page2/index.html`.

```yaml
paginate_path: /blog/page:num
```

The `:num` placeholder is required; a `paginate_path` without it, or a
non-positive `paginate`, fails the build.

## The template page

Pagination renders copies of the `index.html` file in the directory that
`paginate_path` points into — `/page:num` paginates `/index.html`,
`/blog/page:num` paginates `/blog/index.html`. Like Jekyll, it must be an
actual HTML file: a Markdown `index.md` does not qualify, and if no
`index.html` exists a warning is printed and pagination is skipped.

Page 1 **is** the index page at its normal URL; no `page1/` is ever
generated. Pages 2 and up are copies at `paginate_path` with `:num`
substituted.

## The `paginator` object

Available on the template page and its copies (and nowhere else):

| Variable                      | Description                                   |
| ----------------------------- | --------------------------------------------- |
| `paginator.page`              | current page number                           |
| `paginator.per_page`          | posts per page                                |
| `paginator.posts`             | this page's posts, newest first               |
| `paginator.total_posts`       | total number of posts                         |
| `paginator.total_pages`       | total number of pages                         |
| `paginator.previous_page`     | previous page number, or `nil` on page 1      |
| `paginator.previous_page_path`| path to the previous page, or `nil`           |
| `paginator.next_page`         | next page number, or `nil` on the last page   |
| `paginator.next_page_path`    | path to the next page, or `nil`               |

A minimal paginated listing with navigation:

{% raw %}
```liquid
{% for post in paginator.posts %}
  <a href="{{ post.url }}">{{ post.title }}</a>
{% endfor %}

{% if paginator.previous_page %}
  <a href="{{ paginator.previous_page_path }}">Newer</a>
{% endif %}
{% if paginator.next_page %}
  <a href="{{ paginator.next_page_path }}">Older</a>
{% endif %}
```
{% endraw %}

`paginator.previous_page_path` on page 2 is the index page's own URL (for
example `/`), not `/page1/`.

## Differences from Jekyll

- Page URLs are normalized to directory form: with the default
  `paginate_path: /page:num`, links are `/page2/` where Jekyll renders the
  raw `/page2`. Both serve the same `page2/index.html` file.
- [jekyll-paginate-v2](https://github.com/sverrirs/jekyll-paginate-v2)
  features — the `pagination:` front-matter block, paginating collections,
  or per-category/tag pages — are not supported, matching GitHub Pages.
