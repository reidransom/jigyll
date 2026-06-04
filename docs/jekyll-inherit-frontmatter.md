# Jekyll Inherit Frontmatter Plugin - Usage Guide

The `jekyll-inherit-frontmatter` plugin automatically copies front matter fields from base language posts to translated posts, making it easy to maintain multilingual content without duplicating metadata.

## Features

- ✅ Automatically copies specified front matter fields from base language posts to translations
- ✅ Respects explicit overrides in translated posts
- ✅ Configurable default language (defaults to "en")
- ✅ Configurable list of inheritable fields
- ✅ Matches posts by date
- ✅ Works with any number of languages

## Installation

The plugin is built into jigyll. Simply add it to your `_config.yml`:

```yaml
plugins:
  - jekyll-inherit-frontmatter
```

## Configuration

### Basic Configuration

```yaml
# _config.yml
default_lang: en

plugins:
  - jekyll-inherit-frontmatter
```

### Custom Inheritable Fields

By default, the plugin inherits **ALL** fields from the base language post, except:
- `lang` (always excluded)
- `title` (excluded by default, but can be changed)

#### Excluding Specific Fields

To prevent certain fields from being inherited:

```yaml
# _config.yml
default_lang: en

inherit_frontmatter:
  exclude:
    - layout     # Don't inherit layout
    - permalink  # Don't inherit permalink
    # Add any other fields you DON'T want to inherit

plugins:
  - jekyll-inherit-frontmatter
```

#### Explicit Include List

To inherit only specific fields (instead of all):

```yaml
# _config.yml
default_lang: en

inherit_frontmatter:
  fields:
    - author
    - author_url
    - featured_image
    - tags
    - reading_time
    # Only these fields will be inherited

plugins:
  - jekyll-inherit-frontmatter
```

## Usage

### Folder Layout

```
/
├── _config.yml
├── _posts/
│   ├── 2025-11-16-hello-world.md          # English (base)
│   └── es/
│       └── 2025-11-16-hello-world.md      # Spanish (inherits)
```

### English Post (Base Language)

```markdown
---
title: Hello World
layout: post
lang: en
author: Reid Ransom
author_url: https://x.com/reidransom
featured_image: /assets/hello.jpg
tags: [jekyll, i18n]
reading_time: 5 min
---

# Hello World (English)

This is the base content.
```

### Spanish Post (Translated)

```markdown
---
title: ¡Hola Mundo!
layout: post
lang: es
# No need to repeat author, tags, etc. - they're inherited!
---

# ¡Hola Mundo!

Este es el contenido traducido.
```

After the site is built, the Spanish post will automatically have:
- `layout: "post"`
- `author: "Reid Ransom"`
- `author_url: "https://x.com/reidransom"`
- `featured_image: "/assets/hello.jpg"`
- `tags: ["jekyll", "i18n"]`
- `reading_time: "5 min"`

**All fields from the English post are inherited**, except `lang` and `title` (which are excluded by default).

### Accessing Inherited Fields in Templates

```liquid
<!-- _layouts/post.html -->
<article>
  <h1>{{ page.title }}</h1>

  <p class="meta">
    By <a href="{{ page.author_url }}">{{ page.author }}</a>
    • {{ page.reading_time }}
  </p>

  {% if page.featured_image %}
    <img src="{{ page.featured_image }}" class="featured">
  {% endif %}

  {{ content }}
</article>
```

## Overriding Inherited Fields

If you want to override a field in a translated post, just include it in the front matter:

```markdown
---
title: ¡Hola Mundo!
layout: post
lang: es
author: Juan Pérez  # Override the inherited author
---
```

The plugin respects explicit overrides and will not replace them.

## How It Works

1. The plugin runs during the `PostReadSite` phase, after all posts have been loaded
2. It identifies the default language from `_config.yml` (defaults to "en")
3. It builds a map of base language posts, indexed by date
4. For each translated post (non-default language):
   - Finds the corresponding base post by matching the date
   - Copies configured inheritable fields from the base post
   - Skips any fields that already exist in the translated post (respects overrides)

## Example: Multiple Languages

```yaml
# _config.yml
default_lang: en
languages: ["en", "es", "fr", "de"]

plugins:
  - jekyll-inherit-frontmatter
```

Folder structure:
```
_posts/
├── 2025-11-16-hello-world.md     # English (base)
├── es/
│   └── 2025-11-16-hello-world.md # Spanish
├── fr/
│   └── 2025-11-16-hello-world.md # French
└── de/
    └── 2025-11-16-hello-world.md # German
```

All three translations (Spanish, French, German) will inherit from the English base post.

## Benefits

1. **DRY (Don't Repeat Yourself)**: Write metadata once in the base language
2. **Easy Updates**: Change metadata in one place, all translations inherit the update
3. **Consistent**: Ensures metadata consistency across translations
4. **Flexible**: Override any field when needed for specific translations
5. **No Manual Liquid**: No need for `{% assign en = ... %}` blocks in layouts

## Troubleshooting

### Fields not being inherited?

- Ensure both posts have the same date (matched by `YYYY-MM-DD` format)
- Verify both posts have the `lang` field set in front matter
- Check that the base post's `lang` matches your `default_lang` config
- Confirm the field is in the `inherit_frontmatter.fields` list (or using defaults)

### How to debug?

Enable verbose mode when building:

```bash
jigyll build --verbose
```

This will show which plugins are loaded and any errors during the build process.
