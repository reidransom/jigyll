# Jigyll

[![go badge][go-svg]][go-url]
[![Golangci-lint badge][golangci-lint-svg]][golangci-lint-url]


Jigyll is a partially-compatible clone of the [Jekyll](https://jekyllrb.com)
static site generator, written in the [Go](https://golang.org) programming
language. It provides `build` and `serve` commands, with directory watch and
live reload.


## Install

```bash
curl -fsSL https://raw.githubusercontent.com/reidransom/jigyll/main/install.sh | bash
```

The installer also installs Dart Sass if `sass` is not detected on your `PATH`.

[Find other installation methods in the documentation.](https://jigyll.r2ware.app/docs/installation/)


## Source portability audit

Install the independent executable with `go install github.com/reidransom/jigyll/cmd/jigyll-lint@latest` (or `go build -o jigyll-lint ./cmd/jigyll-lint`). It does not run Jigyll or build a site:

```sh
jigyll-lint --policy /path/to/shared.yml /path/to/site
```

One site root is audited per invocation. `_config.yml` supplies `url` and `baseurl`; `--config` selects another site config. `--policy` selects a shared policy and an optional `.jigyll-source-lint.yml` in the site root supplies site overrides (`--site-policy` selects another file). Both policy paths can be outside the site. Example:

```yaml
rules:
  first-party-origin: true
  root-relative: true
  liquid-url: true
  external-host: true
  image-placement: true
  javascript-url: false
allowed_hosts: [fonts.googleapis.com, cdn.example.org]
files:
  - glob: '_includes/**'
    rules: {image-placement: false}
lines:
  - file: index.md
    line: 12
    rules: {root-relative: false}
```

The default-enabled rules are `first-party-origin`, `root-relative`, and `liquid-url`. The opt-in rules are `external-host` (navigation, assets, forms), `image-placement` (local images outside site-root `assets/`), and heuristic `javascript-url`. `allowed_hosts` contains hostnames without schemes; the site's configured host is always internal. A site policy's `allowed_hosts` replaces the shared list. Rule precedence is shared defaults, site defaults, matching shared files, matching site files, matching shared lines, matching site lines; narrower exceptions can enable or disable any rule. File globs and line file paths are relative to the site root, using `/`. A rule ID typo is an error.

The scanner checks HTML/Liquid URL attributes, Markdown links and images, URL-bearing YAML/front matter fields, CSS `url(...)`, and (when enabled) quoted JavaScript URL strings. A filtered `{{ '/path' | relative_url }}` or `absolute_url` expression is portable; an opaque expression is reported. Root-relative local URLs are flagged when `baseurl` is nonempty; external links, fragments, and non-web schemes are not local-path violations. Diagnostics use `file:line: rule: explanation`; exit codes are 0 (clean), 1 (findings), and 2 (usage/I/O/configuration error). This is regex-based source policy, not a proof of rendered or runtime URL correctness. Generated `_site`, dot-directories, `node_modules`, and `vendor` are skipped.

## Development

Development and tests require the standalone Dart Sass `sass` executable on your `PATH`.
Install the pinned version globally with [mise](https://mise.jdx.dev):

```bash
mise use --global github:sass/dart-sass@1.102.0
```

Then confirm the installation with `sass --version`. For other installation methods,
see the official [Sass command-line installation instructions](https://sass-lang.com/install/#command-line).

This project uses [just](https://github.com/casey/just) as a command runner. Run `just` to see available recipes:

```bash
just build      # compile the binary
just buildlinux # cross-compile for linux (amd64 + arm64)
just clean      # remove build artifacts
just deps       # list external transitive dependencies
just imports    # list external direct imports
just install    # install the binary
just lint       # run linter
just race       # compile a race-detector build
just release    # bump patch version, tag, and push
just setup      # install package dependencies and test tools
just test       # run tests
```

## Upstream credit

Jigyll is a fork of [gojekyll](https://github.com/osteele/gojekyll). We gratefully credit its principal contributors: [Oliver Steele](https://github.com/osteele), [Bjørn Erik Pedersen](https://github.com/bep), [Maurits van der Schee](https://github.com/mevdschee), and [Daniil Gentili](https://github.com/danog).

[go-url]: https://github.com/reidransom/jigyll/actions?query=workflow%3A%22Build+Status%22
[go-svg]: https://github.com/reidransom/jigyll/actions/workflows/go.yml/badge.svg
[golangci-lint-url]: https://github.com/reidransom/jigyll/actions?query=workflow%3Agolangci-lint
[golangci-lint-svg]: https://github.com/reidransom/jigyll/actions/workflows/golangci-lint.yml/badge.svg
