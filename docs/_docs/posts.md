---
title: Posts
permalink: /docs/posts/
description: Blogging with the _posts folder — creating posts, tags and categories, excerpts, and drafts.
---

Blogging is baked into Jigyll. You write blog posts as text files and Jigyll
provides everything you need to turn them into a blog.

## The Posts Folder

The `_posts` folder is where your blog posts live. You typically write posts
in Markdown; HTML is also supported.

## Creating Posts

To create a post, add a file to your `_posts` directory with the following
format:

```
YEAR-MONTH-DAY-title.MARKUP
```

Where `YEAR` is a four-digit number, `MONTH` and `DAY` are both two-digit
numbers, and `MARKUP` is the file extension representing the format used in
the file. For example:

```
2011-12-31-new-years-eve-is-awesome.md
2012-09-12-how-to-write-a-blog.md
```

A file in `_posts` whose name doesn't match this pattern is silently
skipped — as in Jekyll, the date prefix is required.

All blog post files must begin with [front matter](/docs/front-matter/),
which is typically used to set a [layout](/docs/layouts/) or other metadata.
For a simple example:

```markdown
---
layout: post
title:  "Welcome to Jigyll!"
---

# Welcome

**Hello world**, this is my first blog post.

I hope you like it!
```

Use the [`post_url`](/docs/liquid/) tag to link to other posts without
having to worry about the URLs breaking when the site permalink style
changes.

## Including images and resources

At some point, you'll want to include images, downloads, or other digital
assets along with your text content. One common solution is to create a
folder in the root of the project directory called something like `assets`,
into which any images, files or other resources are placed. Then, from
within any post, they can be linked to using the site's root as the path:

```markdown
... which is shown in the screenshot below:
![My helpful screenshot](/assets/screenshot.jpg)

... you can [get the PDF](/assets/mydoc.pdf) directly.
```

## Displaying an index of posts

Creating an index of posts on another page is easy thanks to
[Liquid](/docs/liquid/):

{% raw %}
```liquid
<ul>
  {% for post in site.posts %}
    <li>
      <a href="{{ post.url }}">{{ post.title }}</a>
    </li>
  {% endfor %}
</ul>
```
{% endraw %}

Note that the `post` variable only exists inside the `for` loop above. If
you wish to access the currently-rendering page's variables, use the `page`
variable instead.

## Tags and Categories

Tags and categories for a post are defined in the post's front matter with
the `tags` and `categories` keys. Each accepts either a YAML list or a
space-separated string (`tags: classic hollywood` becomes the two tags
`classic` and `hollywood`).

All categories and tags registered in the current site are exposed to Liquid
via `site.categories.CATEGORY` and `site.tags.TAG`, each of which yields the
list of posts in that category or tag. Categories can also be
[incorporated into the post's URL](/docs/permalinks/) via the `:categories`
placeholder; tags cannot be.

> **Differs from Jekyll.** Categories and tags come **only from front
> matter**:
>
> - The singular `category:` and `tag:` keys are ignored — use the plural
>   `categories:` / `tags:`.
> - Directory-based categories are not supported. In Jekyll, a post at
>   `movies/horror/_posts/...` gets `movies` and `horror` as categories;
>   in Jigyll it doesn't.

## Post excerpts

You can access a snippet of a post's content with the `excerpt` variable.
By default this is the first paragraph of content, and it can be customized
by setting `excerpt_separator` in `_config.yml`, or overridden outright with
an `excerpt` key in the post's front matter.

{% raw %}
```liquid
<ul>
  {% for post in site.posts %}
    <li>
      <a href="{{ post.url }}">{{ post.title }}</a>
      {{ post.excerpt }}
    </li>
  {% endfor %}
</ul>
```
{% endraw %}

> **Differs from Jekyll.** `excerpt_separator` is honored only as a global
> setting in `_config.yml` — setting it in an individual document's front
> matter has no effect.

## Drafts

Drafts are posts you're still working on and don't want to publish yet. Put
them in a `_drafts` folder in your site's root, and preview them by running
`jigyll serve` or `jigyll build` with the `--drafts` switch. Draft filenames
don't need a date prefix (`_drafts/a-draft-post.md`); a dateless draft takes
its file's last-modified time as its date, just like Jekyll.

> **Differs from Jekyll.** The `--future` switch and `future:` setting
> decide "future-ness" from the filename date, not a `date:` in front
> matter.
