---
title: Localization
permalink: /docs/localization/
description: Build one Jigyll project into coherent locale-specific sites.
---

Localization is opt-in. Add a `localization` block to `_config.yml`; without it,
Jigyll uses its ordinary single-site build.

```yaml
url: https://example.test
baseurl: /guide
localization:
  default_language: en
  locales:
    en:
      tag: en
      label: English
    de:
      tag: de-DE
      label: Deutsch
      fallbacks: [en]
```

Locale keys are lowercase URL-safe project identifiers. Tags are explicit BCP 47
tags. The default locale owns root routes; other locales are prefixed by their
key. Set `default_language_in_subdir: true` to prefix the default locale too.

## Content editions

Use `lang` to assign an edition and `translation_key` to relate editions. A
missing `lang` uses the default locale. `translation_key` is optional, but must
be a non-empty string when present.

```yaml
---
lang: de
translation_key: getting-started
permalink: /erste-schritte/
---
```

Translation keys are scoped to pages or to one collection. Jigyll never
substitutes default-language page content for a missing edition: unavailable
editions are omitted from routes, lists, feeds, and language selectors.

## Required translations

Set `required_translations` to make coverage of selected locale keys a build
policy. The option is opt-in: omit it, or set it to `[]`, to keep the existing
permissive catalog behavior. Each target is a configured, non-default locale
**key**—not a BCP 47 tag or a label.

In this project, French is required and Danish is optional:

```yaml
localization:
  default_language: en
  locales:
    en: { tag: en, label: English }
    fr: { tag: fr, label: Français }
    da: { tag: da, label: Dansk }
  required_translations: [fr]
```

An included English edition with `translation_key: guide` needs an included
French edition with the same key in the same namespace (pages or one
collection):

```yaml
---
lang: en
translation_key: guide
---
```

```yaml
---
lang: fr
translation_key: guide
---
```

An included Danish `guide` edition is allowed, but it neither creates nor
satisfies the required French edition. A translation set with no included
English edition is also allowed.

The included default-locale edition owns each coverage obligation. For a
deliberate exception, put `translation_exempt` on that edition and name the
required locale key:

```yaml
---
lang: en
translation_exempt: [fr]
---
```

This keyless English document is valid because French is its only required
target. With more required targets, its exemption list must name all of them;
a keyed document may exempt only selected targets. An exemption is not valid
on a non-default edition, for an optional or unknown locale, more than once,
or when an included sibling already provides that locale.

Only included, non-static documents participate. Existing exclusion,
`show_drafts`, `future`, and `unpublished` settings decide inclusion for the
active build, so hidden drafts and future or unpublished documents do not
block a normal build. When the build includes them, they must meet the same
policy. A missing required edition fails the build; Jigyll never publishes
fallback default-language page content at that locale's route.

## Data and messages

Shared `_data` remains visible in every locale. Put locale modules below
`_data/locales/<locale>/`; their filenames become data keys and overlay shared
data through the locale fallback chain.

```text
_data/
  settings.yml
  locales/
    en/messages.yml
    de/messages.yml
    de/settings.yml
```

`messages.yml` supplies the `translate` filter. Message keys are dotted paths;
missing messages fail the build unless `missing_messages: key` is configured.

{% raw %}
```liquid
{{ "nav.home" | translate }}
{{ site.data.settings.title }}
```
{% endraw %}

## Liquid contract

`site.language` is the active locale, `site.languages` is the configured,
deterministically ordered locale list, and `site.default_language` is the
default locale. Locale values expose `key`, `tag`, `label`, `direction`,
`weight`, and `default`.

Pages expose `page.language`, `page.translation_key`, `page.translations`,
`page.all_translations`, and canonical `page.alternates`. `translations`
excludes the current edition; `all_translations` includes it.

Use `translation` for an edition lookup and `localized_url` for a page or known
content route. `localized_url` preserves query strings and fragments. It leaves
external URLs, fragments, and shared assets unchanged, and rejects unknown
internal routes rather than guessing a locale prefix.

{% raw %}
```liquid
{% assign german = page | translation: "de" %}
{% if german %}<a href="{{ german.url }}">Deutsch</a>{% endif %}
<a href="{{ "/about/" | localized_url: "de" }}">Über uns</a>
```
{% endraw %}

## Discovery exports

Add `jigyll-llms-txt` to `plugins` for a single project-level discovery index:

```yaml
plugins: [jigyll-llms-txt]
llms_txt:
  sections:
    - title: Guides
      entries:
        - url: /getting-started/
        - url: /erste-schritte/
```

At `baseurl: /guide`, the index is `/guide/llms.txt`. It groups actual editions
in configured locale order, using H2 labels such as `English — Guides`.
Companions belong to each prepared locale: English `/getting-started/` exports
`/guide/getting-started/index.md`; German `/erste-schritte/` exports
`/guide/de/erste-schritte/index.md`. Prefixing the default language moves the
English companion under `/guide/en/`, but does not move or duplicate the index.
`llms_txt.markdown: false` generates only the index and links to canonical HTML.

Curated URLs name literal routes, not translation identities. Different
translated slugs must be listed separately. A slug missing from one locale
is omitted there only when it exists elsewhere; everywhere-missing or matching
excluded routes fail. Automatic discovery includes actual eligible editions.
Neither selection mode substitutes fallback article content for a missing
edition, and required-translation errors still reject the entire generation.
External curated entries keep their authored destinations in each locale group.

Build/watch stages all locale HTML and discovery artifacts together; preview
serves a completed immutable generation. Failed validation or rendering retains
the previous complete output and index/companion bytes until a valid change
recovers. The separate documentation manifest/corpus snapshot feature remains
unsupported for localized projects, including combined docs/plugin settings.
See [plugin discovery](/docs/plugins/#localized-discovery) for metadata,
curation, and companion rules.

## Publication

Localized builds validate every route before publishing. Output is rendered to
a sibling staging directory and promoted only after the complete generation
succeeds; a failed localized build leaves the previous destination intact.
