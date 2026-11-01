---
title: Command-Line Usage
parent: Build
nav_order: 1
permalink: /docs/usage/
description: Jigyll's command-line interface.
---

Jigyll's CLI mirrors Jekyll's most-used commands:

```bash
jigyll new ./my-site # create a blank site
jigyll build         # build the site in the current directory into _site
jigyll serve         # serve at http://localhost:4000; reload on changes
jigyll clean         # remove the generated _site directory
jigyll help          # list commands
jigyll help build    # help for a specific command
```

## Common flags

These work on both `build` and `serve`:

| Flag | Purpose |
| --- | --- |
| `--source`, `-s` | Source directory (default: current) |
| `--destination`, `-d` | Output directory (default: `./_site`) |
| `--follow-directory-symlinks` | Follow directory symlinks in site sources (off by default; CLI only) |
| `--config` | Comma-separated config files; later files override earlier ones |
| `--drafts`, `-D` | Render posts in `_drafts` |
| `--future` | Render posts with future dates |
| `--unpublished` | Render pages/posts marked `published: false` |
| `--incremental`, `-I` | Rebuild changed files; `jigyll-llms-txt` refreshes the complete generation |
| `--watch`, `-w` | Rebuild on file changes (default: off for `build`, on for `serve`) |
| `--force_polling` | Use a polling file watcher |
| `--quiet`, `-q` / `--verbose`, `-V` | Less / more output |

Flags take precedence over `_config.yml`. Set `JEKYLL_ENV=production` to
build in production mode.

## `build`

Renders the site to the destination directory. `--dry-run` (`-n`) builds
without writing.

## `new`

Creates a minimal, runnable blank site at `PATH`. Use
`jigyll new PATH --theme GIT_URL` to clone a theme into `_theme/` and use it
for the new site. See [Start a new site](/docs/new/) for the generated files
and theme requirements.

## `serve`

Builds the site and serves it locally, rebuilding on change.

| Flag | Purpose |
| --- | --- |
| `--host`, `-H` | Host to bind (default `127.0.0.1`) |
| `--port`, `-P` | Port (default `4000`) |
| `--open-url`, `-o` | Open the site in a browser |

> **Differs from Jekyll.** `serve` renders pages on the fly and does **not**
> write to the filesystem; **live reload is always on**; and watching also
> reloads `_config.yml` and data files. `--baseurl`, `--detach`, and
> `--ssl-*` are not implemented.

With [`jigyll-llms-txt`](/docs/plugins/#builds-and-preview-refreshes) enabled,
watch mode prepares a fresh index and Markdown companions before replacing the
served generation. Invalid updates leave the previous exports available; a
successful refresh updates canonical links and removes obsolete routes.
`build --watch` also uses full generations with this plugin, even when
`--incremental` is set. It validates exports before cleaning the destination,
but does not provide filesystem-wide transactional writes. Documentation
exports retain their explicit incremental-mode restriction.

Fetch advertised canonical paths on the loopback origin to inspect exports
locally; for example, `curl http://127.0.0.1:4000/project/llms.txt`. Keep the
configured public `url` and `baseurl` for canonical citations. Localized projects
likewise serve one aggregate index and actual locale companions from a completed
generation; their disk builds use staged project promotion.

## Jigyll-only commands

> **Jigyll-only.** These have no Jekyll counterpart:

| Command | Purpose |
| --- | --- |
| `jigyll plugins` | List the plugin names the binary recognizes |
| `jigyll render PATH` | Render one file or URL path to stdout |
| `jigyll routes` | Print the site's permalinks and their source files |
| `jigyll variables [PATH]` | Print the site's (or one document's) Liquid variables |
| `jigyll benchmark` | Rebuild repeatedly for ten seconds, with CPU profiling |
| `jigyll version` | Print the version |

## Not implemented

Jekyll's `new-theme`, `doctor`, and `import` commands don't exist, nor do the
`--lsi` and `--limit-posts` flags.
