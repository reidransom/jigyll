---
title: Liquid
permalink: /docs/liquid/
description: The Liquid filters and tags Jigyll supports, what's missing, and the Jigyll-only money filters.
---

Jigyll uses the [Liquid](https://shopify.github.io/liquid/) templating
language to process templates, just like Jekyll. All of the standard Liquid
[tags](https://shopify.github.io/liquid/tags/control-flow/) and
[filters](https://shopify.github.io/liquid/filters/abs/) are supported.

## Error handling

> **Differs from Jekyll.** Liquid error behavior is fixed (Jekyll's
> `liquid:` config options have no effect):
>
> - An **undefined filter** or **unknown tag** is a build error.
> - An **undefined variable** renders as empty, silently.

## Jekyll filters

Jigyll implements Jekyll's filters, including:

| | |
| --- | --- |
| URLs | `relative_url`, `absolute_url` |
| Dates | `date_to_string`, `date_to_long_string`, `date_to_rfc822`, `date_to_xmlschema` |
| Selection | `where`, `where_exp`, `group_by`, `group_by_exp`, `sort`, `uniq`, `sample` |
| Escaping | `xml_escape`, `cgi_escape`, `uri_escape` |
| Text | `markdownify`, `smartify`, `slugify`, `normalize_whitespace`, `number_of_words`, `array_to_sentence_string`, `to_integer` |
| Data | `jsonify`, `inspect` |
| Sass | `scssify` |

See the [Jekyll filter reference](https://jekyllrb.com/docs/liquid/filters/)
for what each does.

> **Differs from Jekyll.** Not supported:
>
> - **`sassify`** — a warning is printed and the input passes through
>   unchanged. `scssify` works.
> - **`find` and `find_exp`** — use `where`/`where_exp` and take `first`.
> - **`where` with `nil`/`empty`** — `where` compares stringified values only;
>   Jekyll 4's nil/empty detection is not implemented.
> - **`slugify` modes `ascii`, `latin`, and `none`** — `default`, `raw`, and
>   `pretty` work.

## Jekyll tags

The Jekyll tags are all available:

- **`include`** / **`include_relative`** — with parameters
  (`{% raw %}{% include note.html content="..." %}{% endraw %}`), theme
  includes, and include-loop detection.
- **`highlight`** — standard blocks use Jekyll's
  `figure.highlight > pre > code` shell, including normalized `language-*` and
  `data-lang` metadata. Chroma supplies token spans and `linenos` internals;
  the optional `linenos` argument works:
  `{% raw %}{% highlight ruby linenos %} ... {% endhighlight %}{% endraw %}`
- **`link`** / **`post_url`** — resolve a source path to its permalink, with
  build-time validation (a missing target fails the build, as in Jekyll).
- **`raw`** and all standard control-flow tags.

> **Differs from Jekyll.** `link` and `post_url` take a literal path only —
> Jekyll 4.5's variable interpolation
> (`{% raw %}{% post_url {{ post.slug }} %}{% endraw %}`) is not supported.

## Money filters

> **Jigyll-only.** These Shopify-style filters have no Ruby Jekyll
> counterpart.

Prices are stored as an **integer number of cents**, following the Shopify
convention — `1000` means $10.00:

{% raw %}
```liquid
{{ 1000 | money }}                          → $10.00
{{ 1000 | money_with_currency }}            → $10.00 USD
{{ 1000 | money_without_currency }}         → 10.00
{{ 1000 | money_without_trailing_zeros }}   → $10
{{ 1099 | money_without_trailing_zeros }}   → $10.99
{{ 123456 | money }}                        → $1,234.56
```
{% endraw %}

Floats and numeric strings are accepted and rounded to the nearest cent;
non-numeric input renders as empty. Configure the currency in `_config.yml`:

```yaml
currency: USD          # code shown by money_with_currency (default: USD)
currency_symbol: "$"   # symbol prefix (default: $)
```

## Localization filters

Localized sites add `translation`, `localized_url`, and `translate`.
`translation` returns a published sibling edition for a locale key.
`localized_url` resolves a page or known content route through the translation
catalog; it preserves query strings and fragments but rejects unknown internal
routes. `translate` reads dotted keys from the active locale's message catalog.

`relative_url` and `absolute_url` keep their existing base-path and deployment
URL behavior; they do not select a locale. See
[Localization](/docs/localization/) for the complete page and site contract.
