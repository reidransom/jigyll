---
title: Assets
parent: Content
nav_order: 6
permalink: /docs/assets/
description: Sass and SCSS conversion in Jigyll — Liquid first, compressed CSS, limited options.
---

Jigyll provides built-in support for [Sass](https://sass-lang.com/). To have
a file converted, give it a `.sass` or `.scss` extension and ***start the
file with two lines of triple dashes***, like this:

```sass
---
---

// start content
.my-definition
  font-size: 1.2em
```

Jigyll treats these files the same as a regular page: the output file is
placed in the same directory it came from. For instance, a file named
`css/styles.scss` is processed and written to your site's destination folder
as `css/styles.css`.

Like Jekyll, Jigyll processes the body after front matter through Liquid before
sending it to the Sass compiler.

> **Differs from Jekyll.** CoffeeScript is not supported. There is no equivalent
> of the `jekyll-coffeescript` plugin; `.coffee` files without front matter are
> copied verbatim as static files.

## Sass/SCSS

Place all your partials in your `sass_dir`, which defaults to
`<source>/_sass`. Place your main SCSS or Sass files where you want them to
be in the output, such as `<source>/css`. Files in the `sass_dir` are the
load path for `@import` — they should *not* have front matter, and they are
not written to the site themselves:

```yaml
sass:
  sass_dir: _sass   # the default
```

> **Differs from Jekyll.** `sass_dir` is the only `sass:` option Jigyll
> honors. Output is **always compressed** — `style` is ignored — and
> `load_paths`, `sourcemap`, Sass warning controls, and deprecation controls are
> not supported. Conversion requires the
> [Dart Sass](https://sass-lang.com/dart-sass/) `sass` executable on your
> `PATH` (the [install script](/docs/installation/) sets this up for you),
> and compiled CSS is cached in `/tmp/jigyll-$USER`.

If your site uses a [theme](/docs/themes/), the theme's `_sass` partials are
on the load path too, and partials in your site's `_sass` override
same-named ones from the theme.

## Example: light and dark colors from data

Store both palettes in a [data file](/docs/datafiles/) and use Liquid to
set Sass variables at build time. This example uses light colors by default
and switches to dark colors when the visitor's system prefers dark mode.
No custom `_config.yml` settings or JavaScript are needed.

Create `_data/colors.yml`:

```yaml
light:
  background: "#ffffff"
  text: "#222222"
dark:
  background: "#181818"
  text: "#eeeeee"
```

Create `assets/main.scss`. Keep the empty front matter so Jigyll processes
the file instead of copying it unchanged:

{% raw %}
```scss
---
---
$light-background: {{ site.data.colors.light.background }};
$light-text: {{ site.data.colors.light.text }};
$dark-background: {{ site.data.colors.dark.background }};
$dark-text: {{ site.data.colors.dark.text }};

:root {
  color-scheme: light;
}

body {
  background-color: $light-background;
  color: $light-text;
}

@media (prefers-color-scheme: dark) {
  :root {
    color-scheme: dark;
  }

  body {
    background-color: $dark-background;
    color: $dark-text;
  }
}
```
{% endraw %}

Link the compiled stylesheet from your layout's `<head>`:

{% raw %}
```html
<link rel="stylesheet" href="{{ '/assets/main.css' | relative_url }}">
```
{% endraw %}

Run `jigyll build`. Liquid reads `site.data.colors` before Sass compiles
`assets/main.scss` to `assets/main.css`. Both palettes are included in the
CSS; the browser chooses one using `prefers-color-scheme`. The
`color-scheme` property also tells the browser how to style native controls.
Rebuild after changing the data file.

Keep Liquid expressions in the entrypoint with front matter. Files in
`_sass` are passed directly to Sass without Liquid processing. With the
legacy `@import` syntax, imported partials can use variables assigned before
the import; with `@use`, configure the module through `with (...)` and its
`!default` variables instead.
