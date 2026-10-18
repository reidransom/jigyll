---
layout: default
title: Jigyll
description: A fast, partially-compatible Jekyll clone written in Go.
nav_exclude: true
---

# Jigyll
{: .fs-9 }

A fast, partially-compatible clone of Jekyll — written in Go, with `build` and
`serve`, directory watch, and live reload. Roughly 20× faster than Ruby Jekyll.
{: .fs-6 .fw-300 }

| &nbsp;                  | Jigyll                               | Jekyll |
| ----------------------- | ------------------------------------ | ------ |
| Stable                  |                                      | ✓      |
| Fast                    | ✓ ([~20×Jekyll]({{ '/docs/benchmarks/' | relative_url }})) |        |
| Template language       | Liquid                               | Liquid |
| SASS                    | ✓                                    | ✓      |
| Jekyll compatibility    | [partial]({{ '/docs/differences/' | relative_url }})          | ✓      |
| Plugins                 | [some]({{ '/docs/plugins/' | relative_url }})                 | yes    |
| Windows support         | ✓                                    | ✓      |
| Implementation language | Go                                   | Ruby   |

<p class="hero-actions">
  <a class="btn btn-primary" href="{{ '/docs/' | relative_url }}">Read the docs</a>
  <a class="btn" href="{{ '/docs/installation/' | relative_url }}">Install</a>
</p>
