---
title: Installation
parent: Getting Started
nav_order: 2
permalink: /docs/installation/
description: Install the Jigyll binary via mise, Homebrew, Scoop, Docker, or the install script.
---

Jigyll ships as a single Go binary — there is **no Ruby, RubyGems, or Bundler**
to install. Pick whichever method fits your platform.

## Install script (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/reidransom/jigyll/main/install.sh | bash
```

## mise (macOS / Linux / Windows)

Install the latest release and make `jigyll` available globally with
[`mise`](https://mise.jdx.dev):

```bash
mise use --global github:reidransom/jigyll
```

## Homebrew (macOS / Linux)

```bash
brew tap reidransom/tap
brew install jigyll
```

This pulls in [`dart-sass`](https://formulae.brew.sh/formula/dart-sass)
automatically, so SCSS/Sass works out of the box.

## Scoop (Windows)

```powershell
scoop bucket add reidransom https://github.com/reidransom/scoop-bucket
scoop install jigyll
```

## Docker

```bash
docker run --rm -v "$PWD:/site" ghcr.io/reidransom/jigyll build
```

## From source

```bash
go install github.com/reidransom/jigyll@latest
```

Building from source requires the standalone Dart Sass `sass` executable on
your `PATH` for SCSS support. Install the pinned version globally with
[mise](https://mise.jdx.dev):

```bash
mise use --global github:sass/dart-sass@1.102.0
```

Confirm the installation with `sass --version`.

## Optional: shell completion

Add the appropriate command to your shell profile (such as `.bashrc` or
`.zshrc`):

```bash
# Bash:
eval "$(jigyll --completion-script-bash)"

# Zsh:
eval "$(jigyll --completion-script-zsh)"
```
