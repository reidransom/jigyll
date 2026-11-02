---
title: Themes
permalink: /docs/themes/
description: Using Jekyll themes with Jigyll — the _theme folder, Bundler fallback, and what's not supported.
---

Themes package layouts, includes, and stylesheets in a way that can be
overridden by your site's content. A theme contributes its `_layouts`,
`_includes`, `_sass`, and `assets` directories to your site's build.

Jigyll supports two separate theme mechanisms:

- `theme` selects a local `_theme/<name>` directory or a Bundler theme.
- `remote_theme` downloads a pinned GitHub theme source archive.

Do not configure both keys. A build with both fails with:

```text
_config.yml cannot specify both theme and remote_theme
```

## Local themes

Activate a local or Bundler theme with the `theme` key in `_config.yml`:

```yaml
theme: minima
```

> **Differs from Jekyll.** There is no gem installation step. Jigyll resolves
> `theme` in two ways, in order:
>
> 1. **A local `_theme` folder** — if `<source>/_theme/<theme-name>/` exists,
>    it is used. This is the Jigyll-native way to vendor a theme: copy (or
>    submodule) the theme's files there.
>
> 2. **Bundler fallback** — otherwise, if `bundle` is on your `PATH`, Jigyll
>    runs `bundle show <theme-name>` and uses the gem's directory. This lets an
>    existing Jekyll project with a `Gemfile` keep working, but it requires a
>    Ruby toolchain.
>
> If neither works, the build fails with an error.

### Installing a local theme

For example, install Minima as a local theme:

```sh
mkdir -p _theme
git clone https://github.com/jekyll/minima.git _theme/minima
```

Then set `theme: minima` in your site's `_config.yml`.

### Scaffolding with a local theme

`jigyll new` can clone and select a theme in one step:

```sh
jigyll new my-site --theme GIT_URL
```

This clones the repository to `_theme/<theme-name>/` and writes
`theme: <theme-name>` to `my-site/_config.yml`. The generated site has no local
`_layouts/default.html`, so the selected theme's layout is used. The URL must
resolve to a theme directory name and the repository must provide that layout.

## Pinned GitHub remote themes

Use `remote_theme` only with an immutable GitHub commit:

```yaml
remote_theme: just-the-docs/just-the-docs@394d6c0ec33852f8e593145d21344a955e908acb
```

The exact accepted syntax is `owner/repository@<40-character-hex-SHA>`.
Branches, tags, shortened SHAs, omitted revisions, URLs, and non-GitHub hosts
are intentionally unsupported. Jigyll constructs the archive request itself
for `https://codeload.github.com`; configuration cannot choose another host.

The first build downloads and validates the archive, so it requires network
access. Validated themes are cached at
`<user-cache-dir>/jigyll/themes/<sha256-normalized-spec>/` (`$XDG_CACHE_HOME`
on Linux). Subsequent builds use that immutable cache entry without Git,
submodule checkout, or another download.

## What a theme provides

Jigyll reads these directories from the theme, with your site's own files
always taking precedence:

- `_layouts` — a page's `layout` is looked up in your site's `_layouts`
  first, then the theme's.
- `_includes` — `{% raw %}{% include %}{% endraw %}` searches your site's
  `_includes` first, then the theme's.
- `_sass` — the theme's partials are on the Sass load path; a same-named
  partial in your site's `_sass` wins.
- `assets` — theme assets are output unless your site has a file at the
  same path.

> **Differs from Jekyll.** A theme's `_data` directory (Jekyll 4.3+) and a
> theme-bundled `_config.yml` (Jekyll 4.0+) are ignored, the theme's
> gemspec `runtime_dependencies` are not auto-loaded as plugins (list the
> [emulated plugins](/docs/plugins/) you need explicitly), and there are no
> `theme.*` Liquid variables.

## Overriding theme defaults

To replace layouts or includes in your theme, make a copy in your
`_layouts` or `_includes` directory of the specific file you wish to
modify, or create the file from scratch giving it the same name as the file
you wish to override.

For example, if your selected theme has a `page` layout, you can override
the theme's layout by creating your own `page` layout in the `_layouts`
directory (that is, `_layouts/page.html`).

To modify a stylesheet, also copy the theme's main Sass file into the
`_sass` directory in your site's source. Your theme's styles can be
included in your stylesheet using the `@import` directive:

{% raw %}
```css
@import "theme-partial";
```
{% endraw %}

## Creating themes

Jekyll's `jekyll new-theme` scaffolding and RubyGems publishing workflow
don't apply to Jigyll. A Jigyll theme is just a directory with `_layouts`,
`_includes`, `_sass`, and `assets` folders — anything that follows that
shape (including any existing Jekyll theme's source) can be dropped into
`_theme/<name>/` and used directly.
