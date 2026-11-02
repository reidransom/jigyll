---
layout: default
title: Jigyll
description: A fast, partially-compatible Jekyll clone written in Go.
nav_exclude: true
---

A fast, partially-compatible clone of Jekyll — written in Go, with `build` and
`serve`, directory watch, and live reload. Roughly 20× faster than Ruby Jekyll.

| &nbsp;                  | Jigyll                               | Jekyll |
| ----------------------- | ------------------------------------ | ------ |
| Stable                  |                                      | ✓      |
| Fast                    | ✓ ([~20×Jekyll]({{ '/docs/benchmarks/' | relative_url }})) |        |
| Template language       | Liquid                               | Liquid |
| SASS                    | ✓                                    | ✓      |
| Jekyll compatibility    | [partial]({% link _docs/differences.md %}) | ✓      |
| Plugins                 | [some]({% link _docs/plugins.md %})     | yes    |
| Windows support         | ✓                                    | ✓      |
| Implementation language | Go                                   | Ruby   |

{% capture homepage_actions %}
{% include components/link-button.html text="Read the docs" href="/docs/" %}
{% include components/link-button.html text="Install" href="/docs/installation/" variant="secondary" %}
{% endcapture %}
<div class="hero-actions not-content">{{ homepage_actions | strip_newlines }}</div>

{% include jigyll/site-links.html %}
