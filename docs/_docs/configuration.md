---
title: Configuration
parent: Build
nav_order: 2
permalink: /docs/configuration/
description: Jigyll's _config.yml options, defaults, and how they differ from Jekyll's.
---

Jigyll reads its settings from a `_config.yml` file in your site's root
directory, exactly like Jekyll. Options can also be set with command-line
flags, which take precedence over the config file.

> **Differs from Jekyll.** Only `_config.yml` is read — there is no
> `_config.toml` support. And Jigyll is **strict about types**: a wrong YAML
> type for a supported key (a list where a string is expected, or vice versa)
> is a build error, where Jekyll often coerces or ignores it. Unknown keys are
> fine — they simply become `site.*` variables.

## Multiple config files

Pass a comma-separated list to `--config`; later files override earlier ones:

```bash
jigyll build --config _config.yml,_config.production.yml
```

The `JEKYLL_CONFIG` environment variable is used as the file list when
`--config` is not given.

## Default configuration

Jigyll's defaults, which your `_config.yml` overrides:

```yaml
# Where things are
source:       .
destination:  ./_site
layouts_dir:  _layouts
data_dir:     _data
includes_dir: _includes
collections:
  posts:
    output:   true

# Handling Reading
include:              [".htaccess"]
exclude:              ["Gemfile", "Gemfile.lock", "node_modules", "vendor/bundle/", "vendor/cache/", "vendor/gems/", "vendor/ruby/"]
keep_files:           [".git", ".svn"]
markdown_ext:         "markdown,mkdown,mkdn,mkd,md"

# Plugins
plugins:   []

# Conversion
excerpt_separator: "\n\n"
incremental: false

# Serving
port:    4000
host:    127.0.0.1
baseurl: "" # does not include hostname

# Outputting
permalink: date
timezone:  null

verbose: false
sass:
  sass_dir: _sass
```

Other supported keys: `theme`, `url`, `show_drafts`, `future`, `unpublished`,
`defaults` (see below). `permalink` accepts the `date`, `pretty`, `ordinal`,
and `none` styles or a custom template string.

## Discovery and documentation exports

[`jigyll-llms-txt`](/docs/plugins/#jigyll-native-discovery-jigyll-llms-txt)
is opt-in through the `plugins` list. Its optional `llms_txt` map accepts
`title`, `description`, `markdown` (default `true`), and ordered `sections`
containing `title` and `entries`; entries accept `url`, `title`, and
`description`. There is no separate `enabled` switch. Settings alone do not
activate the plugin.

```yaml
url: https://example.com
title: Example
plugins:
  - jigyll-llms-txt
llms_txt:
  markdown: true
```

An absolute HTTP(S) canonical `url` and nonempty effective title are required.
Standalone discovery supports `baseurl`; local curated routes are written
before that prefix, while generated canonical destinations include it.
See the plugin guide for selection, exclusions, curation, and companion paths.
Localized discovery and generation-safe reload support are not shipped yet.

[`documentation:`](/docs/static-documentation/) independently enables
documentation manifest/corpus/article exports. It does **not** generate or
reserve `llms.txt`; add the plugin explicitly to retain discovery on migration.
Documentation still requires full non-localized builds with empty `baseurl`
and rejects incremental mode. `documentation.enabled: false` disables those
exports, not an independently enabled discovery plugin.


## Front matter defaults

The `defaults` key works like Jekyll's — an array of scope/values pairs:

```yaml
defaults:
  - scope:
      path: ""        # empty string matches all files
      type: "posts"   # optional: pages, posts, drafts, or a collection name
    values:
      layout: "default"
```

Front matter in the file itself always wins. Beyond that, conflicts between
matching scopes are resolved by **precedence**, as in Jekyll:

1. The scope with the **longer** `path` wins, regardless of config order.
2. On equal-length paths, a scope with a `type` beats one without.
3. On a final tie, the **later** entry in `defaults` wins.

Matching scopes contribute their values by merging (different keys coexist;
overlapping keys are decided by the precedence rules above).

`path` may contain globs using `*` and `**`:

```yaml
defaults:
  - scope: { path: "assets/**" }
    values: { sitemap: false, image: true }
  - scope: { path: "section/*/special.md" }
    values: { layout: special }
```

`*` matches a single path segment (it does not cross `/`); `**` matches any
number of segments. As in Jekyll, a glob scope also matches any file **under**
a directory the glob expands to, so `section/*` applies to `section/a/deep/f.md`
(via the `section/a` directory).

`type` filters which documents a scope applies to. A scope without `type`
matches every document, including **static files** (files without front
matter). A scope with `type` matches only documents of that type; static files
outside any collection are untyped, so a `type: pages` scope does **not** apply
to them (e.g. `scope: { type: pages } values: { published: false }` will not
drop static files). Static files inside a collection have that collection's
name as their type.

See also [Static Files](/docs/static-files/) for surfacing default values on
static files via `site.static_files`.

## Environments

The `JEKYLL_ENV` environment variable is exposed to Liquid as
`jekyll.environment`, defaulting to `development`:

```bash
JEKYLL_ENV=production jigyll build
```

{% raw %}
```liquid
{% if jekyll.environment == "production" %}
  {% include analytics.html %}
{% endif %}
```
{% endraw %}

Jigyll also honors `JEKYLL_URL`, which overrides the configured `url`.

## Markdown and Liquid options

> **Differs from Jekyll.** Jekyll's `markdown:`, `kramdown:`, and `liquid:`
> configuration sections are **ignored**. Jigyll has a single built-in
> [goldmark renderer](/docs/rendering-process/) that is not configurable, and
> Liquid error handling is fixed: undefined filters and tags are build errors,
> undefined variables render as empty. `error_mode`, `strict_variables`, and
> `strict_filters` have no effect.

## Sass options

Sass/SCSS conversion is always on (see
[jekyll-sass-converter](/docs/plugins/)). Of Jekyll's `sass:` options, only
`sass_dir` is honored:

```yaml
sass:
  sass_dir: _sass   # the default
```

> **Differs from Jekyll.** Output is **always compressed** — `style` is ignored.
> `load_paths`, `sourcemap`, Sass warning controls, and deprecation controls are
> not supported. Conversion requires the
> [Dart Sass](https://sass-lang.com/dart-sass/) `sass` executable on your
> `PATH`. Compiled CSS is cached in `/tmp/jigyll-$USER`, not
> `./.sass-cache`.

## Localization

Native localization is configured through `localization:`. It validates locale
keys, BCP 47 tags, fallback chains, and locale metadata before Jigyll renders
any output. Localized builds reject `incremental: true`, rebuild the complete
project on source changes, and publish one staged generation at a time.

See [Localization](/docs/localization/) for configuration, content-edition,
data, message, and URL-filter examples.

### Required translation targets

`localization.required_translations` is an optional sequence of locale keys:

```yaml
localization:
  default_language: en
  locales:
    en: { tag: en, label: English }
    fr: { tag: fr, label: Français }
    da: { tag: da, label: Dansk }
  required_translations: [fr]
```

Each value must be a unique configured locale key other than
`default_language`. Tags such as `fr-FR` and labels such as `Français` are not
accepted. An omitted or empty sequence preserves permissive localization
behavior; it does not validate `translation_exempt` front matter.

When the sequence is non-empty, each included, non-static default-locale
edition must use `translation_key` or exempt every required target. A keyed
edition needs an included sibling for every required target unless that target
is explicitly exempted. Use effective front matter on the default-locale
edition:

```yaml
---
translation_exempt: [fr]
---
```

`translation_exempt` must be a sequence of unique locale-key strings. Every
entry must be a configured required target; unknown, default, and optional
locale keys are rejected. Exemptions on non-default editions are rejected, as
are exemptions for a target that already has an included sibling. See
[Localization](/docs/localization/) for normal, exempted, optional-locale, and
active-build examples.

## Incremental regeneration

As in Jekyll, pass `--incremental` (`-I`) or set `incremental: true` to only
regenerate documents whose sources (or included templates/layouts) changed.

## Unsupported keys

`safe`, `whitelist`, `highlighter`, `lsi`, `limit_posts`,
and `profile` have no effect. Plugins are configured with `plugins:` (the
legacy `gems:` alias also works) — never a Gemfile.
