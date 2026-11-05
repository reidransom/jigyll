---
title: Start a New Site
permalink: /docs/new/
description: Create a Jigyll site with a repository-reference theme or a blank scaffold.
---

Install a theme from a repository reference without Git:

```sh
jigyll new my-docs --theme owner/docifly@latest
cd my-docs
jigyll build
```

`owner/docifly` is a syntax placeholder, not a claim that a release exists.
Shorthand references use GitHub. `latest` selects the provider's latest
published release, not its default branch. Use an exact tag or a full
40-character commit SHA to select another revision:

```sh
jigyll new my-docs --theme owner/docifly@v1.2.3
jigyll new my-docs --theme owner/docifly@FULL_COMMIT_SHA
```

Replace `FULL_COMMIT_SHA` with the recorded full hexadecimal commit. For a
Forgejo repository, include the instance URL explicitly:

```sh
jigyll new my-docs --theme https://src.r2ware.dev/owner/project@v1.2.3
```

The owner, repository, and tag above are placeholders. Jigyll resolves the
reference and internally downloads a `.tar.gz` source archive at the resolved
commit; there is no arbitrary archive-URL, theme-name, or checksum option.
It removes the archive's enclosing directory and installs ordinary files at
`_theme/<repository-name>/`. The generated `_config.yml` selects that name
(for example, `theme: docifly`), without copying the theme's configuration.
The archive must provide a regular `_layouts/default.html`; the generated
site does not shadow it with a starter layout. Unsafe paths, links, special
files, `.git` metadata, and archives exceeding the installer limits are rejected.

After installation, builds consume local theme files without network access
or Git. Themes shipping precompiled CSS and browser scripts need no Node,
npm, or Sass build step. Themes that require Sass compilation still need
their usual compiler.

### Provenance and repeatability

`_theme/<repository-name>.source.json` records the requested source,
repository URL, resolved tag (when applicable), full resolved commit, archive
URL, and observed SHA-256 digest. To reinstall the same source revision in a
new destination, replace `latest` or the tag with that `resolved_commit`:

```sh
jigyll new my-docs-pinned --theme owner/docifly@FULL_COMMIT_SHA
```

For Forgejo, retain the explicit instance URL. Tags and `latest` can change.
The digest identifies the downloaded bytes; it is not verification against an
independently trusted expected checksum or proof of publisher identity.
Provider-generated archives of the same commit are not promised to remain
byte-identical. Provenance is an installation record, not an updater or a
build-time network dependency. See [Themes](/docs/themes/) for the distinction
between installed local themes and pinned GitHub `remote_theme`.

## Start blank

Create a minimal, runnable blank site with no Ruby toolchain:

```sh
jigyll new my-site
cd my-site
jigyll serve
```

Open the address printed by `serve` (by default, <http://localhost:4000>). Build
static output instead with `jigyll build`.

The new site contains:

```text
my-site/
├── _config.yml
├── _data/
├── _drafts/
├── _includes/
├── _layouts/default.html
├── _posts/
├── _sass/
├── assets/
├── index.md
└── .gitignore
```

The blank scaffold's `index.md` uses the included `default` layout, so it
builds immediately. Blank is the only default: Jigyll has no `--blank` flag,
and creating a site does not create a `Gemfile`, invoke Bundler, or require
Ruby.

## Start with a Git URL

The existing Git installer remains available:

```sh
jigyll new my-site --theme GIT_URL
```

This clones the repository to `_theme/<theme-name>/`, selects it in
`_config.yml`, and requires `_layouts/default.html`. The Git URL must end in
a safe directory name (such as `my-theme.git`). Git is needed for this
legacy installation path, not for subsequent local-theme builds.

## Destination safety

`jigyll new` refuses a non-empty destination. It stages the complete scaffold
beside the target and publishes it only after all writes and theme validation
succeed.
