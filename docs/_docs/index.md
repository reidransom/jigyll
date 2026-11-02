---
title: Documentation
permalink: /docs/
description: Get started with Jigyll, a fast Jekyll clone written in Go.
---

**Jigyll** is a partially-compatible clone of the
[Jekyll](https://jekyllrb.com) static site generator, written in
[Go](https://golang.org). It provides `build` and `serve` commands, with
directory watching and live reload — and runs roughly **20× faster** than
Ruby Jekyll on the sites it supports.

Because Jigyll aims for Jekyll compatibility, most of what you already know
about Jekyll applies here. Where behavior differs, these docs call it out
explicitly — see [Differences from Jekyll](/docs/differences/).

## Quick start

```bash
jigyll build       # build the current directory into _site
jigyll serve       # serve at http://localhost:4000 with live reload
```

Head to [Installation](/docs/installation/) to get the binary, then
[Usage](/docs/usage/) for the commands and flags.
