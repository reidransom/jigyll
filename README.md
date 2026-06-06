# Jigyll

<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->

[![All Contributors](https://img.shields.io/badge/all_contributors-5-orange.svg?style=flat-square)](#contributors-)

<!-- ALL-CONTRIBUTORS-BADGE:END -->

[![go badge][go-svg]][go-url]
[![Golangci-lint badge][golangci-lint-svg]][golangci-lint-url]
[![Coveralls badge][coveralls-svg]][coveralls-url]
[![Go Report Card badge][go-report-card-svg]][go-report-card-url]
[![MIT License][license-svg]][license-url]

**Jigyll** is a fork of [**gojekyll**](https://github.com/osteele/gojekyll), which was created by Oliver Steele ([@osteele](https://github.com/osteele)) and is maintained by Daniil Gentili ([@danog](https://github.com/danog)). Jigyll is not affiliated with or endorsed by the original authors. All original work remains under their copyright; see [LICENSE](./LICENSE).  

Jigyll is a partially-compatible clone of the [Jekyll](https://jekyllrb.com)
static site generator, written in the [Go](https://golang.org) programming
language. It provides `build` and `serve` commands, with directory watch and
live reload.

| &nbsp;                  | Jigyll                                  | Jekyll | Hugo                         |
| ----------------------- | ----------------------------------------- | ------ | ---------------------------- |
| Stable                  |                                           | ✓      | ✓                            |
| Fast                    | ✓<br>([~20×Jekyll](./docs/benchmarks.md)) |        | ✓                            |
| Template language       | Liquid                                    | Liquid | Go, Ace and Amber templates  |
| SASS                    | ✓                                         | ✓      | ✓                            |
| Jekyll compatibility    | [partial](#current-limitations)           | ✓      |                              |
| Plugins                 | [some](./docs/plugins.md)                 | yes    | shortcodes, theme components |
| Windows support         | ✓                                         | ✓      | ✓                            |
| Implementation language | Go                                        | Ruby   | Go                           |

<!-- TOC -->

- [Usage](#usage)
- [Installation](#installation)
  - [Homebrew (macOS / Linux)](#homebrew-macos--linux)
  - [Scoop (Windows)](#scoop-windows)
  - [Quick Install (macOS / Linux)](#quick-install-macos--linux)
  - [Docker](#docker)
  - [Binary Downloads](#binary-downloads)
  - [From Source](#from-source)
- [[Optional] Install command-line autocompletion](#optional-install-command-line-autocompletion)
- [Development](#development)
- [Status](#status)
  - [Current Limitations](#current-limitations)
  - [Other Differences](#other-differences)
  - [Feature Checklist](#feature-checklist)
- [Troubleshooting](#troubleshooting)
- [Contributors](#contributors)
- [Attribution](#attribution)
- [Related](#related)
- [License](#license)

<!-- /TOC -->

## Usage

```bash
jigyll build       # builds the site in the current directory into _site
jigyll serve       # serve the app at http://localhost:4000; reload on changes
jigyll help
jigyll help build
```

## Installation

### Homebrew (macOS / Linux)

```bash
brew tap reidransom/tap
brew install jigyll
```

This pulls in [`dart-sass`](https://formulae.brew.sh/formula/dart-sass)
automatically, so SCSS/Sass works out of the box.

### Scoop (Windows)

```powershell
scoop bucket add reidransom https://github.com/reidransom/scoop-bucket
scoop install jigyll
```

This pulls in the [`sass`](https://scoop.sh/#/apps?q=sass) package (Dart Sass)
automatically, so SCSS/Sass works out of the box.

### Quick Install (macOS / Linux)

The installer downloads the `jigyll` binary and, unless `sass` is already on your
PATH, a matching [Dart Sass](https://github.com/sass/dart-sass):

```bash
curl -fsSL https://raw.githubusercontent.com/reidransom/jigyll/main/install.sh | sh
```

Pin a specific release with `VERSION`, or change the install location with
`INSTALL_DIR`:

```bash
curl -fsSL https://raw.githubusercontent.com/reidransom/jigyll/main/install.sh | VERSION=v1.0.1 sh
```

Prefer to read before running? Download and inspect it first:

```bash
curl -fsSL https://raw.githubusercontent.com/reidransom/jigyll/main/install.sh -o install.sh
less install.sh && sh install.sh
```

### Docker

You can use `jigyll` with the official `reidransom/jigyll` image, for example to build the site in the current directory into `_site`:

```bash
docker run --user $UID:$GID -v $PWD:/app --pull always --rm -it reidransom/jigyll build -s /app
```

Another example, serve the website in the current directory on `http://localhost:4000`, automatically reloading on changes:

```bash
# Linux (host networking):
docker run --user $UID:$GID -v $PWD:/app --pull always --network host --rm -it reidransom/jigyll serve -s /app
```

On Docker Desktop (macOS / Windows), `--network host` does not reach the host, so
publish the port and bind the server to all interfaces inside the container:

```bash
docker run --user $UID:$GID -v $PWD:/app --pull always -p 4000:4000 --rm -it reidransom/jigyll serve -s /app -H 0.0.0.0
```

### Binary Downloads

1. Linux, Mac OS and Windows binaries for amd64, armv6/v7, armv8, riscv64 are available from the [releases
   page](https://github.com/reidransom/jigyll/releases).
   SCSS/Sass support requires the [Dart Sass](https://github.com/sass/dart-sass)
   `sass` executable on your PATH (the [Quick Install](#quick-install-macos--linux)
   script sets this up for you; otherwise see [From Source](#from-source) for how to
   install it).
2. [Optional] **Themes**. To use a theme, you need to install Ruby and
   [bundler](http://bundler.io/). Create a `Gemfile` that lists the theme., and
   run `bundle install`. The [Jekyll theme
   instructions](https://jekyllrb.com/docs/themes/) provide more detail, and
   should work for Jigyll too.

### From Source

Pre-requisites:

1. **Install go** (1) via [Homebrew](https://brew.sh): `brew install go`; or (2)
   [download](https://golang.org/doc/install#tarball).
2. Install the Dart Sass executable:
   - On macOS: `brew install sass/sass/sass`
   - On Linux: see item (2) under [Binary Downloads](#binary-downloads) or install via your package manager.
3. See item (2) under [Binary Downloads](#binary-downloads) for theme support.

Then run:

```bash
go install github.com/reidransom/jigyll@latest
```


## [Optional] Install command-line autocompletion

Add this to your `.bashrc` or `.zshrc`:

```bash
# Bash:
eval "$(jigyll --completion-script-bash)"
# Zsh:
eval "$(jigyll --completion-script-zsh)"
```

## Development

This project uses [just](https://github.com/casey/just) as a command runner. Run `just` to see available recipes:

```bash
just build      # compile the binary
just buildlinux # cross-compile for linux (amd64 + arm64)
just clean      # remove build artifacts
just install    # install the binary
just lint       # run linter
just release    # bump patch version, tag, and push
just test       # run tests
```

## Status

This project works on the GitHub Pages sites that I and other contributors care
about. It looks credible on a spot-check of other Jekyll sites.

### Current Limitations

Missing features:

- Pagination
- Math
- Plugin system. ([Some individual plugins](./docs/plugins.md) are emulated.)
- Liquid filter `sassify` is not implemented
- Liquid is run in strict mode: undefined filters and variables are errors.
- Missing markdown features:
  - [attribute list definitions](https://kramdown.gettalong.org/syntax.html#attribute-list-definitions) (ALDs, i.e. `{:refname: .class}` reusable sets; inline attribute lists `{: .class #id}` on headings are supported)
  - [`markdown="span"`, `markdown="block"`](https://kramdown.gettalong.org/syntax.html#html-blocks)
  - Markdown configuration options

Also see the [detailed status](#feature-status) below.

### Other Differences

These will probably not change:

By design:

- Plugins must be listed in the config file, not a Gemfile.
- The wrong type in a `_config.yml` file – for example, a list where a string is
  expected, or vice versa – is generally an error.
- Server live reload is always on.
- `serve --watch` (the default) reloads the `_config.yml` and data files too.
- `serve` generates pages on the fly; it doesn't write to the file system.
- Files are cached in `/tmp/jigyll-${USER}`, not `./.sass-cache`

Upstream:

- Markdown:
  - `<` and `>` inside markdown is interpreted as HTML. For example, `This is
<b>bold</b>` renders as <b>bold</b>. This behavior matches the [Markdown
    spec](https://daringfireball.net/projects/markdown/syntax#html), but differs
    from Jekyll's default Kramdown processor.
  - The autogenerated id of a header that includes HTML is computed from the
    text of the title, ignoring its attributes. For example, the id of `## Title
(<a href="https://example.com/path/to/details">ref</a>))` is `#title-ref`,
    not `#title-https-example-path-to-details-ref`.
  - Autogenerated header ids replace punctuation by the hyphens, rather than the
    empty string. For example, the id of `## Either/or` is `#either-or` not
    `#eitheror`; the id of `## I'm Lucky` is `#i-m-lucky` not `#im-lucky`.

Muzukashii:

- An extensible plugin mechanism – support for plugins that aren't compiled into
  the executable.

### Feature Checklist

- [ ] Content
  - [x] Front Matter
  - [x] Posts
  - [x] Static Files
  - [x] Variables
  - [x] Collections
  - [x] Data Files
  - [ ] Assets
    - [ ] Coffeescript
    - [x] Sass/SCSS
- [ ] Customization
  - [x] Templates
    - [ ] Jekyll filters
      - [ ] `scssify`
      - [x] everything else
    - [x] Jekyll tags
  - [x] Includes
  - [x] Permalinks
  - [ ] Pagination
  - [ ] Plugins – partial; see [here](./docs/plugins.md)
  - [x] Themes
  - [x] Layouts
- [x] Server
  - [x] Directory watch
- [ ] Commands
  - [x] `build`
    - [x] `--source`, `--destination`, `--drafts`, `--future`, `--unpublished`
    - [x] `--incremental`, `--watch`, `--force_polling`, `JEKYLL_ENV=production`
    - [ ] `--baseurl`, `--config`, `--lsi`
    - [ ] `--limit-posts`
  - [x] `clean`
  - [x] `help`
  - [x] `serve`
    - [x] `--open-uri`, `--host`, `--port`
    - [x] `--incremental`, `–watch`, `--force_polling`
    - [ ] `--baseurl`, `--config`
    - [ ] `--detach`, `--ssl`-\* – not planned
  - [ ] `doctor`, `import`, `new`, `new-theme` – not planned
- [x] Windows

## Troubleshooting

If the error is "403 API rate limit exceeded", you are probably building a
repository that uses the `jekyll-github-metadata` gem. Try setting the
`JEKYLL_GITHUB_TOKEN`, `JEKYLL_GITHUB_TOKEN`, or `OCTOKIT_ACCESS_TOKEN`
environment variable to the value of a [GitHub personal access
token][personal-access-token] and trying again.

[personal-access-token]: https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token

## Contributors

Thanks goes to these wonderful people ([emoji key](https://allcontributors.org/docs/en/emoji-key)):

<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<table>
  <tr>
    <td align="center"><a href="https://code.osteele.com/"><img src="https://avatars.githubusercontent.com/u/674?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Oliver Steele</b></sub></a><br /><a href="https://github.com/osteele/gojekyll/commits?author=osteele" title="Code">💻</a> <a href="#design-osteele" title="Design">🎨</a> <a href="https://github.com/osteele/gojekyll/commits?author=osteele" title="Documentation">📖</a> <a href="#ideas-osteele" title="Ideas, Planning, & Feedback">🤔</a> <a href="#infra-osteele" title="Infrastructure (Hosting, Build-Tools, etc)">🚇</a> <a href="#maintenance-osteele" title="Maintenance">🚧</a> <a href="#projectManagement-osteele" title="Project Management">📆</a> <a href="https://github.com/osteele/gojekyll/pulls?q=is%3Apr+reviewed-by%3Aosteele" title="Reviewed Pull Requests">👀</a> <a href="https://github.com/osteele/gojekyll/commits?author=osteele" title="Tests">⚠️</a></td>
    <td align="center"><a href="https://bep.is/"><img src="https://avatars.githubusercontent.com/u/394382?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Bjørn Erik Pedersen</b></sub></a><br /><a href="https://github.com/osteele/gojekyll/commits?author=bep" title="Documentation">📖</a></td>
    <td align="center"><a href="https://tqdev.com/"><img src="https://avatars.githubusercontent.com/u/1288217?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Maurits van der Schee</b></sub></a><br /><a href="https://github.com/osteele/gojekyll/commits?author=mevdschee" title="Code">💻</a></td>
    <td align="center"><a href="https://daniil.it/"><img src="https://avatars.githubusercontent.com/u/7339644?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Daniil Gentili</b></sub></a><br /><a href="https://github.com/osteele/gojekyll/commits?author=danog" title="Code">💻</a></td>
    <td align="center"><a href="http://cameronelliott.com/"><img src="https://avatars.githubusercontent.com/u/868689?v=4?s=100" width="100px;" alt=""/><br /><sub><b>Cameron Elliott</b></sub></a><br /><a href="#ideas-cameronelliott" title="Ideas, Planning, & Feedback">🤔</a></td>
  </tr>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

This project follows the
[all-contributors](https://github.com/all-contributors/all-contributors)
specification. [Contributions of any kind welcome](./CONTRIBUTING.md)!

## Attribution

Jigyll uses these libraries:

| Package                                                                        | Author(s)                                        | Usage                                                      | License                                 |
| ------------------------------------------------------------------------------ | ------------------------------------------------ | ---------------------------------------------------------- | --------------------------------------- |
| [github.com/jaschaephraim/lrserver](https://github.com/jaschaephraim/lrserver) | Jascha Ephraim                                   | Live Reload                                                | MIT License                             |
| [github.com/kyokomi/emoji](https://github.com/kyokomi/emoji)                   | kyokomi                                          | `jemoji` plugin emulation                                  | MIT License                             |
| [github.com/osteele/liquid](https://github.com/osteele/liquid)                 | yours truly                                      | Liquid processor                                           | MIT License                             |
| [github.com/pkg/browser](https://github.com/pkg/browser)                       | [pkg](https://github.com/pkg)                    | `serve --open-url` option                                  | BSD 2-clause "Simplified" License       |
| [github.com/radovskyb/watcher](https://github.com/radovskyb/watcher)           | Benjamin Radovsky                                | Polling file watch (`--force_polling`)                     | BSD 3-clause "New" or "Revised" License |
| [github.com/yuin/goldmark](https://github.com/yuin/goldmark)                   | Yusuke Inuzuka                                   | Markdown processing (CommonMark + GFM + extensions)        | MIT License                             |
| [github.com/sass/dart-sass](https://github.com/sass/dart-sass)                 | Listed [here](https://github.com/sass/dart-sass) | The reference implementation of Sass, written in Dart.     | MIT License                             |
| [github.com/tdewolff/minify](https://github.com/tdewolff/minify)               | Taco de Wolff                                    | CSS minimization                                           | MIT License                             |
| [github.com/bep/godartsass](https://github.com/bep/godartsass)                 | Drew Wells                                       | Go API backed by the native Dart Sass Embedded executable. | MIT License                             |
| [github.com/alecthomas/kingpin/v2](https://github.com/alecthomas/kingpin)      | Alec Thomas                                      | command-line arguments                                     | MIT License                             |
| [github.com/alecthomas/chroma](https://github.com/alecthomas/chroma)           | Alec Thomas                                      | Syntax highlighter                                         | MIT License                             |
| [gopkg.in/yaml.v2](https://github.com/go-yaml/yaml)                            | Canonical                                        | YAML support                                               | Apache License 2.0                      |

In addition, the following pieces of text were taken from Jekyll and its plugins.
They are used under the terms of the MIT License.

| Source                                                                          | Use                  | Description            |
| ------------------------------------------------------------------------------- | -------------------- | ---------------------- |
| [Jekyll template documentation](https://jekyllrb.com/docs/templates/)           | test cases           | filter examples        |
| `jekyll help` command                                                           | `jigyll help` text | help text              |
| [`jekyll-feed` plugin](https://github.com/jekyll/jekyll-feed)                   | plugin emulation     | `feed.xml` template    |
| [`jekyll-redirect-from` plugin](https://github.com/jekyll/jekyll-redirect-from) | plugin emulation     | redirect page template |
| [`jekyll-sitemap` plugin](https://github.com/jekyll/jekyll-redirect-from)       | plugin emulation     | sitemap template       |
| [`jekyll-seo-tag` plugin](https://github.com/jekyll/jekyll-redirect-from)       | plugin emulation     | feed template          |

The theme for in-browser error reporting was adapted from facebookincubator/create-react-app.

The gopher image in the `testdata` directory is from [Wikimedia
Commons](https://commons.wikimedia.org/wiki/File:Gophercolor.jpg). It is used
under the [Creative Commons Attribution-Share Alike 3.0 Unported
license](https://creativecommons.org/licenses/by-sa/3.0/deed.en).

In addition to being totally and obviously inspired by Jekyll and its plugins,
Jekyll's solid _documentation_ was indispensible --- especially since I wanted
to implement Jekyll as documented, not port its source code. The [Jekyll
docs](https://jekyllrb.com/docs/home/) were always open in at least one tab
during development.

## Related

[Hugo](https://gohugo.io) is the pre-eminent Go static site generator. It isn't
Jekyll-compatible (-), but it's highly polished, performant, and productized
(+++).

[Liquid](https://github.com/osteele/liquid) is a pure Go implementation of
Liquid templates. I created it in order to use in this project.

[Jekyll](https://jekyllrb.com), of course.

## License

MIT

[coveralls-url]: https://coveralls.io/r/reidransom/jigyll
[coveralls-svg]: https://img.shields.io/coveralls/reidransom/jigyll.svg?branch=main
[license-url]: https://github.com/reidransom/jigyll/blob/main/LICENSE
[license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
[go-url]: https://github.com/reidransom/jigyll/actions?query=workflow%3A%22Build+Status%22
[go-svg]: https://github.com/reidransom/jigyll/actions/workflows/go.yml/badge.svg
[golangci-lint-url]: https://github.com/reidransom/jigyll/actions?query=workflow%3Agolangci-lint
[golangci-lint-svg]: https://github.com/reidransom/jigyll/actions/workflows/golangci-lint.yml/badge.svg
[go-report-card-url]: https://goreportcard.com/report/github.com/reidransom/jigyll
[go-report-card-svg]: https://goreportcard.com/badge/github.com/reidransom/jigyll
