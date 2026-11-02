---
title: Static Files
permalink: /docs/static-files/
description: Files without front matter — copied verbatim and listed in site.static_files.
---

A static file is a file that does not contain any front matter. These
include images, PDFs, and other un-rendered content. They are copied to the
destination verbatim.

Static files are accessible in Liquid via `site.static_files` and contain
the following metadata:

| Variable | Description |
| --- | --- |
| `file.path` | The site-relative path to the file, e.g. `/assets/img/image.jpg` |
| `file.modified_time` | The time the file was last modified |
| `file.name` | The string name of the file, e.g. `image.jpg` |
| `file.basename` | The string basename of the file, e.g. `image` |
| `file.extname` | The extension name of the file, e.g. `.jpg` |

Note that in the above table, `file` can be anything — it's an arbitrary
variable used in your own logic, such as a `for` loop, not a global site or
page variable:

{% raw %}
```liquid
{% for file in site.static_files %}
  {{ file.path }}
{% endfor %}
```
{% endraw %}

Which files count as static is affected by the `include:` and `exclude:`
configuration keys, and files or directories whose names start with `.`,
`#`, `~`, or `_` are skipped unless listed under `include:` — the same rules
as Jekyll.

## Front matter values for static files

Static files don't have front matter of their own, but you can attach
default values to them through the `defaults` configuration, just like in
Jekyll. Because static files outside any collection are **untyped**, only
scopes **without a `type`** apply to them (see
[Front matter defaults](/docs/configuration/#front-matter-defaults)).

For example, tag every file under `assets/img` with `image: true`, then filter
`site.static_files` with `where`:

```yaml
defaults:
  - scope:
      path: "assets/img"
    values:
      image: true
```

{% raw %}
```liquid
{% assign images = site.static_files | where: "image", true %}
{% for file in images %}
  {{ file.path }}
{% endfor %}
```
{% endraw %}

A common use is to exclude a group of static files from the sitemap:

```yaml
defaults:
  - scope: { path: "assets" }
    values: { sitemap: false }
```

The default values surface through the static-file drop alongside the
metadata fields above. The metadata fields (`path`, `name`, `basename`,
`modified_time`, `extname`, `collection`) are computed from the file itself
and cannot be overridden by defaults — defaults only add additional keys.
