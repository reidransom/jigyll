---
title: Collections
permalink: /docs/collections/
description: Grouping related content into collections, and which collection options Jigyll supports.
---

Collections are a great way to group related content like members of a team
or talks at a conference.

## Setup

To use a Collection you first need to define it in your `_config.yml`. For
example, here's a collection of staff members:

```yaml
collections:
  - staff_members
```

You can optionally specify metadata for your collection by defining
`collections` as a mapping instead of a sequence:

```yaml
collections:
  staff_members:
    people: true
```

When defining a collection as a sequence, its pages will not be rendered by
default — `output: true` must be specified on the collection, which requires
defining the collection as a mapping. See [Output](#output) below.

> **Differs from Jekyll.** `collections_dir` is not supported — collection
> folders always live at the root of the source directory. Custom sorting
> (`sort_by:` and `order:` metadata) is also not supported: the built-in
> `posts` collection is sorted by date (newest first), and every other
> collection keeps filesystem order. Sort in Liquid with the `sort` filter
> if you need a specific order.

## Add content

Create a corresponding folder (e.g. `<source>/_staff_members`) and add
documents. Front matter is processed if it exists, and everything after the
front matter is pushed into the document's `content` attribute. If no front
matter is provided, Jigyll considers the file a
[static file](/docs/static-files/) and its contents don't undergo further
processing.

Regardless of whether front matter exists or not, Jigyll writes to the
destination directory (e.g. `_site`) only if `output: true` has been set in
the collection's metadata.

For example, here's how you would add a staff member to the collection set
above. The filename is `./_staff_members/jane.md` with the following
content:

```markdown
---
name: Jane Doe
position: Developer
---
Jane has worked on Jigyll for the past *five years*.
```

The folder must be named identically to the collection you defined in your
`_config.yml` file, with the addition of the preceding `_` character.

## Output

Now you can iterate over `site.staff_members` on a page and output the
content for each staff member. Similar to posts, the body of the document is
accessed using the `content` variable:

{% raw %}
```liquid
{% for staff_member in site.staff_members %}
  <h2>{{ staff_member.name }} - {{ staff_member.position }}</h2>
  <p>{{ staff_member.content | markdownify }}</p>
{% endfor %}
```
{% endraw %}

If you'd like Jigyll to create a rendered page for each document in your
collection, set the `output` key to `true` in your collection metadata in
`_config.yml`:

```yaml
collections:
  staff_members:
    output: true
```

You can link to the generated page using the `url` attribute:

{% raw %}
```liquid
{% for staff_member in site.staff_members %}
  <h2>
    <a href="{{ staff_member.url }}">
      {{ staff_member.name }} - {{ staff_member.position }}
    </a>
  </h2>
  <p>{{ staff_member.content | markdownify }}</p>
{% endfor %}
```
{% endraw %}

Documents in collections you create are accessible via Liquid irrespective
of whether they're output, and unrendered documents keep working as data.

## Permalinks

You can override the global [permalink](/docs/permalinks/) for a collection
with `permalink:` in the collection's configuration. The default for a
collection is `/:collection/:path:output_ext`.

```yaml
collections:
  staff_members:
    output: true
    permalink: /staff/:name/
```

## Liquid Attributes

### Collections

Collections are also available under `site.collections`, with the metadata
you specified in your `_config.yml` (if present) and the following
information:

| Variable | Description |
| --- | --- |
| `label` | The name of your collection, e.g. `my_collection`. |
| `docs` | An array of [documents](#documents). |
| `files` | An array of static files in the collection. |
| `relative_directory` | The path to the collection's source directory, relative to the site source. |
| `directory` | The full path to the collection's source directory. |
| `output` | Whether the collection's documents will be output as individual files. |

Note that the `posts` collection is hard-coded into Jigyll, just as in
Jekyll. It exists whether you have a `_posts` directory or not — something
to note when iterating through `site.collections`.

### Documents

In addition to any front matter provided in the document's corresponding
file, each document has the following attributes:

| Variable | Description |
| --- | --- |
| `content` | The (unrendered) content of the document — everything after the front matter's terminating `---`. |
| `path` | The full path to the document's source file. |
| `relative_path` | The path to the document's source file relative to the site source. |
| `url` | The URL of the rendered document. The file is only written to the destination when its collection has `output: true`. |
| `collection` | The name of the document's collection. |
| `date` | The document's date. |
