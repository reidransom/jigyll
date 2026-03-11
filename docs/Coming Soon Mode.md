# Coming Soon Mode
Add a `coming_soon` config variable that, when `true`, replaces the entire site with a single "coming soon" landing page\.
## Current State
* `_config.yml` has site\-wide settings; no coming\-soon toggle exists today\.
* Three layouts: `default.html`, `page.html`, `post.html` — all in `_layouts/`\.
* Top\-level pages: `index.html`, `blog.html`, `websites.html`, `privacy-policy.md`, `404.html`\.
* Blog posts live in `_posts/`\.
* Navigation menu defined in `_data/menu.yml` and rendered via `_includes/menu.html`\.
## Proposed Changes
### 1\. Add `coming_soon` variable to `_config.yml`
Add `coming_soon: true` \(set to `false` to restore the full site\)\.
### 2\. Create a new layout `_layouts/coming_soon.html`
A minimal, self\-contained layout that:
* Extends `default.html` so it inherits `<head>`, SEO tags, favicon, and styles\.
* Shows a centered "Coming Soon" message with the site title/logo\.
* Hides the nav menu, footer, and all other content\.
* Optionally includes a brief tagline or contact email from `_config.yml`\.
### 3\. Gate `default.html` to redirect all pages
At the top of `_layouts/default.html`, add a Liquid check:
* If `site.coming_soon` is true **and** the current page is **not** the homepage \(`page.url != "/"`\), render a meta\-refresh redirect to `/` instead of the normal page content\.
* This ensures blog posts, `blog.html`, `websites.html`, `privacy-policy.md`, and any future pages all redirect to the homepage without needing per\-page changes\.
### 4\. Update `index.html` to use the coming\-soon layout conditionally
Change the frontmatter of `index.html`:
* Use `layout: coming_soon` when `site.coming_soon` is true, otherwise `layout: default`\.
* Since Jekyll frontmatter doesn't support conditionals, instead keep `layout: default` and handle the swap inside `default.html`: when `site.coming_soon` is true and `page.url == "/"`, include the coming\-soon content block instead of `{{ content }}`\.
### 5\. Disable RSS feed and sitemap in coming\-soon mode
In `_config.yml`, document that when `coming_soon: true` you should also add `jekyll-feed` and `jekyll-sitemap` to the `exclude` or disable them, **or** add a `defaults` block that sets `sitemap: false` on all pages/posts\. This prevents search engines from indexing content that isn't publicly visible\.
## Summary of File Changes
* `_config.yml` — add `coming_soon: true`
* `_layouts/coming_soon.html` — new file \(minimal landing page content\)
* `_layouts/default.html` — add Liquid conditional to redirect non\-homepage pages and swap in coming\-soon content for the homepage
* No changes needed to `index.html`, `blog.html`, `post.html`, `page.html`, or any posts — the gate in `default.html` handles everything centrally\.
