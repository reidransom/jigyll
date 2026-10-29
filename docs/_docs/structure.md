---
title: Directory Structure
parent: Site Structure
nav_order: 1
permalink: /docs/structure/
description: The layout of a Jigyll site's source directory, and where caches live.
---

A basic Jigyll site usually looks something like this:

```
.
├── _config.yml
├── _data
│   └── members.yml
├── _drafts
│   ├── begin-with-the-crazy-ideas.md
│   └── on-simplicity-in-technology.md
├── _includes
│   ├── footer.html
│   └── header.html
├── _layouts
│   ├── default.html
│   └── post.html
├── _posts
│   ├── 2025-10-29-why-every-programmer-should-play-nethack.md
│   └── 2026-04-26-barcamp-boston-4-roundup.md
├── _sass
│   ├── _base.scss
│   └── _layout.scss
├── _site
└── index.html # can also be an 'index.md' with valid front matter
```

> **Differs from Jekyll.** No `.jekyll-cache`, `.sass-cache`, or
> `.jekyll-metadata` is ever created in your project. Jigyll's build cache
> lives outside the source tree, in `jigyll-$USER` under the system temp
> directory (`$TMPDIR` on macOS, `/tmp` on Linux). Set the
> `JIGYLL_DISABLE_CACHE` environment variable to turn it off.

An overview of what each of these does:

| File / Directory | Description |
| --- | --- |
| `_config.yml` | Stores [configuration](/docs/configuration/) data. Many of these options can be specified from the command line, but it's easier to specify them here so you don't have to remember them. |
| `_drafts` | Drafts are unpublished posts. Learn how to [work with drafts](/docs/posts/#drafts). |
| `_includes` | These are the partials that can be mixed and matched by your layouts and posts to facilitate reuse. The Liquid tag `{% raw %}{% include file.ext %}{% endraw %}` can be used to include the partial in `_includes/file.ext`. |
| `_layouts` | These are the templates that wrap posts and pages. Layouts are chosen on a page-by-page basis in the [front matter](/docs/front-matter/). The Liquid tag `{% raw %}{{ content }}{% endraw %}` is used to inject content into the web page. |
| `_posts` | Your dynamic content: blog posts. The naming convention of these files is important, and must follow the format `YEAR-MONTH-DAY-title.MARKUP`. The [permalinks](/docs/permalinks/) can be customized for each post, but the date and markup language are determined solely by the file name. |
| `_data` | Well-formatted site data should be placed here. Jigyll autoloads `.yml`, `.yaml`, `.json`, and `.csv` files in this directory (subfolders are [namespaced by folder name](/docs/datafiles/)), accessible via `site.data`. |
| `_sass` | These are Sass partials that can be imported into your `main.scss`, which is then processed into a single stylesheet. See [assets](/docs/assets/). |
| `_site` | This is where the generated site is placed when you run `jigyll build`. It's a good idea to add this to your `.gitignore` file. Note that `jigyll serve` renders entirely in memory and never writes here. |
| `index.html` or `index.md`, and other HTML/Markdown files | Provided that the file has a [front matter](/docs/front-matter/) section, it will be transformed by Jigyll. |
| Other files/folders | Except for the special cases listed above, every other directory and file — `css`, `images`, `favicon.ico`, and so forth — is copied verbatim to the generated site as a [static file](/docs/static-files/). |

Every file or directory beginning with `.`, `_`, `#`, or `~` in the source
directory is excluded from the destination. Such paths have to be explicitly
listed in the config file under the `include` key to make sure they're
copied over:

```yaml
include:
  - _pages
  - .htaccess
```

## Directory symlinks

An included directory symlink fails the build by default. To publish content
from a linked directory, explicitly enable traversal for each build or serve
command:

```bash
ln -s /home/me/notes ./notes
jigyll build --follow-directory-symlinks
jigyll serve --follow-directory-symlinks
```

The linked files keep paths relative to the link: `notes/topic.md` is treated
as a site file at that path. Normal front matter, collection, `include`, and
`exclude` rules still apply. Excluded links are not traversed. File symlinks
do not require the flag.

Watch mode follows changes in linked directories, including changes to the
link itself. Broken links and directory cycles report errors. Generated output
is excluded even when a link points to it.

Linked directories can expose files outside the site source. Link only
content intended for publication, or exclude private files explicitly.
