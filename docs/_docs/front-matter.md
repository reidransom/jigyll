---
title: Front Matter
permalink: /docs/front-matter/
description: The YAML front matter block — predefined variables, custom variables, and Jigyll's strict parsing.
---

Any file that contains a [YAML](https://yaml.org/) front matter block will
be processed by Jigyll as a special file. The front matter must be the first
thing in the file and must take the form of valid YAML set between
triple-dashed lines. Here is a basic example:

```yaml
---
layout: post
title: Blogging Like a Hacker
---
```

Between these triple-dashed lines, you can set predefined variables (see
below for a reference) or even create custom ones of your own. These
variables will then be available to access using Liquid tags both further
down in the file and also in any [layouts](/docs/layouts/) or
[includes](/docs/includes/) that the page or post in question relies on.

If you want to use Liquid tags and variables but don't need anything in your
front matter, just leave it empty. The set of triple-dashed lines with
nothing in between will still get Jigyll to process your file. (This is
useful for things like [CSS and RSS feeds](/docs/assets/).)

> **Differs from Jekyll.** Two strictness notes:
>
> - **Malformed front matter YAML always fails the build.** Jekyll's
>   `strict_front_matter` option is ignored — there is no lenient mode.
> - **A file without front matter is never processed** — it's copied
>   through as a [static file](/docs/static-files/) even if it's Markdown,
>   unless the [jekyll-optional-front-matter plugin](/docs/plugins/) is
>   enabled.

## Predefined Global Variables

There are a number of predefined global variables that you can set in the
front matter of a page or post.

| Variable | Description |
| --- | --- |
| `layout` | If set, this specifies the [layout](/docs/layouts/) file to use. Use the layout file name without the file extension. Layout files must be placed in the `_layouts` directory. |
| `permalink` | If you need your processed blog post URLs to be something other than the site-wide style (default `/year/month/day/title.html`), then you can set this variable and it will be used as the final URL. |
| `published` | Set to false if you don't want a specific post to show up when the site is generated. Preview unpublished pages with `jigyll serve --unpublished`. |

## Custom Variables

You can also set your own front matter variables you can access in Liquid.
For instance, if you set a variable called `food`, you can use that in your
page:

{% raw %}
```liquid
---
food: Pizza
---

<h1>{{ page.food }}</h1>
```
{% endraw %}

## Predefined Variables for Posts

These are available out-of-the-box to be used in the front matter for a
post.

| Variable | Description |
| --- | --- |
| `date` | A date here overrides the date from the name of the post. This can be used to ensure correct sorting of posts. A date is specified in the format `YYYY-MM-DD HH:MM:SS +/-TTTT`; hours, minutes, seconds, and timezone offset are optional. |
| `categories` | Instead of placing posts inside of folders, you can specify one or more categories that the post belongs to. Can be specified as a YAML list or a space-separated string. **The singular `category` key is not supported** — see [Posts](/docs/posts/). |
| `tags` | Similar to categories, one or multiple tags can be added to a post as a YAML list or a space-separated string. (Singular `tag` is likewise not supported.) |

## Front matter defaults

If you don't want to repeat frequently used front matter variables over and
over, define [defaults](/docs/configuration/) for them in `_config.yml` and
only override them where necessary. This works both for predefined and
custom variables.

> **Differs from Jekyll.** Defaults scopes have one limitation: `path` is
> matched as a plain prefix (no globs). See
> [Configuration](/docs/configuration/).
