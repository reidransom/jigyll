---
title: Data Files
permalink: /docs/datafiles/
description: Loading custom data from the _data directory, and which formats Jigyll reads.
---

In addition to the [built-in variables](/docs/variables/) available from
Jigyll, you can specify your own custom data that can be accessed via
[Liquid](/docs/liquid/).

Jigyll loads data files from the `_data` directory and makes them available
as `site.data`. This lets you avoid repetition in your templates and set
site-specific options without changing `_config.yml`.

## Supported formats

> **Differs from Jekyll.** Jigyll reads **YAML** (`.yml`, `.yaml`), **JSON**
> (`.json`), and **CSV** (`.csv`) files. Jekyll additionally supports TSV;
> Jigyll does not. One more caveat:
>
> - **CSV files parse into rows of strings** (`site.data.members[0][1]`),
>   not header-keyed maps — Jekyll's header-row convention
>   (`member.name`) does not apply. The `csv_reader`/`tsv_reader` config
>   options are not supported.

Subdirectories of `_data` are namespaced by folder name, just as in Jekyll:
`_data/orgs/jekyll.yml` is available as `site.data.orgs.jekyll`.

## Example: list of members

In `_data/members.yml`:

```yaml
- name: Eric Mill
  github: konklone

- name: Parker Moore
  github: parkr
```

This data can be accessed via `site.data.members` (the file's *basename*
determines the variable name, so avoid two data files with the same basename
but different extensions).

You can now render the list of members in a template:

{% raw %}
```liquid
<ul>
{% for member in site.data.members %}
  <li>
    <a href="https://github.com/{{ member.github }}">
      {{ member.name }}
    </a>
  </li>
{% endfor %}
</ul>
```
{% endraw %}

## Example: accessing a specific author

Pages and posts can also access a specific data item. In `_data/people.yml`:

```yaml
dave:
    name: David Smith
    twitter: DavidSilvaSmith
```

The author can then be specified as a page variable in a post's front matter:

{% raw %}
```liquid
---
title: sample post
author: dave
---

{% assign author = site.data.people[page.author] %}
<a rel="author"
  href="https://twitter.com/{{ author.twitter }}"
  title="{{ author.name }}">
    {{ author.name }}
</a>
```
{% endraw %}
