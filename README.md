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
