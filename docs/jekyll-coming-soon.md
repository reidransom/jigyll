# jekyll-coming-soon

A gojekyll plugin that replaces your entire site with a single "coming soon" landing page.

## Usage

Add the plugin to your `_config.yml`:

```yaml
plugins:
  - jekyll-coming-soon

coming_soon: true
```

When `coming_soon: true`:

- **All non-homepage pages** are unpublished (not written to the build output).
- **The homepage (`/`)** is replaced with a coming-soon landing page.
- **`jekyll-feed` and `jekyll-sitemap`** are automatically suppressed.

Set `coming_soon: false` (or remove it) to restore the full site.

## Custom Layout

By default, the plugin renders a built-in landing page with the site title and a "Coming Soon" message.

To customize the appearance, create `_layouts/coming_soon.html` in your site. When this layout exists, the plugin assigns it to the homepage instead of using the built-in fallback. Your layout has full access to Liquid variables (`site.title`, etc.).

## Configuration

| Key            | Type   | Description                                       |
|----------------|--------|---------------------------------------------------|
| `coming_soon`  | `bool` | Enable/disable coming soon mode. Default: `false` |
| `title`        | `string` | Used in the built-in fallback page heading.     |
