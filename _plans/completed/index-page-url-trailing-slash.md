# `page.url` for `index.html` pages should collapse to the directory (`/`), matching Ruby Jekyll

## Problem

For an HTML page named `index.html`, gojekyll reports `page.url` as the full
`…/index.html` path. Ruby Jekyll (and GitHub Pages) instead report the
**containing directory with a trailing slash**, treating `index.html` as a
directory index.

| Source file        | gojekyll `page.url` | Ruby Jekyll `page.url` |
|--------------------|---------------------|------------------------|
| `index.html`       | `/index.html`       | `/`                    |
| `sub/index.html`   | `/sub/index.html`   | `/sub/`                |
| `about.html`       | `/about.html`       | `/about.html` ✓        |

Non-index pages already match; only `index` pages diverge.

### Reproduction

```sh
mkdir -p site/sub && cd site
printf 'url: x\n' > _config.yml
printf -- '---\n---\nurl={{ page.url }}\n' > index.html
printf -- '---\n---\nurl={{ page.url }}\n' > sub/index.html
gojekyll build -q && grep -rh url= _site
# gojekyll:    url=/index.html   url=/sub/index.html
# Ruby Jekyll: url=/             url=/sub/
```

The **output file paths are already correct** in both engines
(`_site/index.html`, `_site/sub/index.html`) — only the value exposed to
templates as `page.url` is wrong.

## Impact

`page.url` feeds anything that builds canonical / share / comparison URLs:

- **SEO includes** (e.g. minima-style `seo.html`): `page.url | absolute_url`
  produces a canonical `<link>`, `og:url`, and JSON-LD `url` of
  `https://example.com/index.html` instead of `https://example.com/`. Search
  engines and social scrapers then see a non-canonical home URL.
- **Active-nav / current-page checks**: templates that compare
  `page.url == '/'` (a common idiom — gojekyll's own bundled `seo.html` JSON-LD
  block does exactly this) silently fail for the home page, because `page.url`
  is `/index.html`.
- **`jekyll-sitemap` already normalizes this** independently — its emitted
  `<loc>` for the home page is `/`, which is *inconsistent* with the
  `/index.html` that `page.url` reports in the same build. Aligning `page.url`
  removes that internal contradiction.

## Root Cause

`pages/permalinks.go`:

```go
const DefaultPermalinkPattern = "/:path:output_ext"
```

`:path` is `"/" + utils.TrimExt(relpath)`, which for `index.html` is `/index`,
so the default pattern yields `/index.html`. There is no special-casing for
HTML pages whose basename is `index`.

Ruby Jekyll's `Jekyll::Page#template` special-cases this: when the page is HTML
and its basename is `index`, the permalink template is the directory
(`"/:path/"`) rather than `"/:path/:basename:output_ext"`. See
<https://github.com/jekyll/jekyll/blob/master/lib/jekyll/page.rb> (`#template`
/ `#index?`).

## Proposed Fix

Collapse a trailing `index.html` to its directory in `computePermalink`, but
**only for the default pattern** (an explicit `permalink:` in front matter must
be honored verbatim, as Ruby Jekyll does) and only for HTML output:

```go
func (p *page) computePermalink(vars map[string]string) (src string, err error) {
	explicit := p.fm.String("permalink", "") != ""
	pattern := p.fm.String("permalink", DefaultPermalinkPattern)
	if pat, found := PermalinkStyles[pattern]; found {
		pattern = pat
	}
	// …existing SafeReplaceAllStringFunc expansion…
	permalink := utils.URLPathClean("/" + s)

	// Ruby Jekyll treats an HTML `index.html` as a directory index: its URL is
	// the containing directory with a trailing slash (`/`, `/sub/`), not
	// `…/index.html`. The output file is still written as index.html.
	if !explicit && p.OutputExt() == ".html" {
		if permalink == "/index.html" {
			permalink = "/"
		} else if strings.HasSuffix(permalink, "/index.html") {
			permalink = strings.TrimSuffix(permalink, "index.html") // keep trailing slash
		}
	}
	return permalink, nil
}
```

### Why this is safe for file output

`site/write.go` (`WriteDoc`) already appends `index.html` to any extension-less
URL:

```go
if !d.IsStatic() && filepath.Ext(rel) == "" {
    rel = filepath.Join(rel, "index.html")
}
```

So a permalink of `/` writes `_site/index.html` and `/sub/` writes
`_site/sub/index.html` — byte-identical output to today. Only `URL()` changes.

## Verification / Test Cases

After the change, confirm:

1. `page.url` → `/` for root `index.html`, `/sub/` for `sub/index.html`,
   `/about.html` unchanged.
2. Output tree unchanged: `_site/index.html`, `_site/sub/index.html` still
   written.
3. `gojekyll serve` still resolves `GET /` and `GET /sub/` (route lookup in
   `site/site.go` already joins `index.html` onto directory URL paths —
   `s.Routes[filepath.Join(urlpath, "index.html")]`).
4. An explicit `permalink: /index.html` in front matter is still emitted
   verbatim (not collapsed).
5. A page with a non-HTML output ext named `index` (e.g. `index.xml`) is **not**
   collapsed.

## Notes

- Discovered while converting an HTML5 UP template to a Jekyll site tested with
  gojekyll but intended to stay 100% portable to Ruby Jekyll. The only
  cross-engine output difference in the whole site was this home-page canonical
  URL.
