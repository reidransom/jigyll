---
title: Start a New Site
permalink: /docs/new/
description: Create a blank Jigyll site or start with a vendored Git theme.
---

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

## Start with a theme

Clone a Git theme and select it in the generated configuration:

```sh
jigyll new my-site --theme GIT_URL
```

This clones the repository to `_theme/<theme-name>/`, adds
`theme: <theme-name>` to `_config.yml`, and lets the theme supply
`_layouts/default.html`. The theme URL must end in a safe directory name (such
as `my-theme.git`) and the cloned theme must include that default layout. `git`
is required only for this themed form.

`jigyll new` refuses a non-empty destination. It stages the complete scaffold
beside the target and publishes it only after all writes and theme validation
succeed.
