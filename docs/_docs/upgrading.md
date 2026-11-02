---
title: Upgrading
permalink: /docs/upgrading/
description: How to upgrade the Jigyll binary.
---

Jigyll is a single binary versioned with semver tags — there is no Bundler,
no `Gemfile.lock`, and no `gem update`. Upgrade through whatever channel you
[installed](/docs/installation/) with:

```bash
# Homebrew
brew upgrade jigyll

# Scoop
scoop update jigyll

# Docker
docker pull ghcr.io/reidransom/jigyll

# Install script — re-run it
curl -fsSL https://raw.githubusercontent.com/reidransom/jigyll/main/install.sh | bash

# From source
go install github.com/reidransom/jigyll@latest
```

Check what you're running with:

```bash
jigyll version
```

Release notes are published on the [GitHub releases
page](https://github.com/reidransom/jigyll/releases).

## Upgrading from Jekyll

Migrating a site *from Ruby Jekyll to Jigyll* usually means deleting the
`Gemfile`, moving the plugin list into `_config.yml` under `plugins:`, and
checking the [differences page](/docs/differences/) for anything your site
relies on. Jekyll's own major-version upgrade guides (0→2, 2→3, 3→4) don't
apply here.
