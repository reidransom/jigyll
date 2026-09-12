---
title: Rendering Process
parent: Build
nav_order: 3
permalink: /docs/rendering-process/
description: How Jigyll renders a page, and how its goldmark Markdown differs from kramdown.
---

Like Jekyll, Jigyll renders each file in stages, with the output of one stage
feeding the next:

1. **Front matter** is read and stripped from the file.
2. **Liquid** expressions in the body are evaluated.
3. **Converters** run, based on the file's extension — Markdown becomes HTML,
   Sass/SCSS becomes CSS. Markdown inside a `.html` file remains untouched.
4. **Layouts** wrap the result: the `layout` from the page's front matter is
   rendered around the content, and each layout's own `layout:` key chains to
   its parent, Russian-doll style. Layout bodies are processed as Liquid only,
   never as Markdown.

Files without front matter are copied verbatim as static files (unless the
[jekyll-optional-front-matter plugin](/docs/plugins/) is enabled).

> **Differs from Jekyll.** Sass/SCSS files skip the Liquid stage entirely —
> the body after front matter goes straight to the Sass compiler. In Jekyll, a
> Sass file is a page like any other and may contain Liquid.

## Markdown: goldmark, not kramdown

Markdown is rendered by [goldmark](https://github.com/yuin/goldmark), a
CommonMark engine, with GitHub Flavored Markdown extensions — tables,
strikethrough, autolinks, task lists — plus definition lists and footnotes.
The renderer is **not configurable**: `markdown:` and `kramdown:` settings in
`_config.yml` are ignored.

Most Markdown renders identically. The differences that matter:

- **Raw `<` and `>` are treated as HTML.** `This is <b>bold</b>` renders as
  bold text. This matches the Markdown spec but differs from kramdown's
  default escaping.
- **No smart quotes.** kramdown typographically curls quotes by default;
  goldmark leaves them straight. (The `smartify` Liquid filter is available
  when you want that.)
- **Math is not supported.** `$...$` and `$$...$$` pass through as literal
  text.
- **No table of contents.** kramdown's TOC marker is not expanded. Use a
  Liquid loop or hand-written links instead.

## Header IDs

Headings get auto-generated `id` attributes, but the algorithm differs from
kramdown's:

- Punctuation becomes a **hyphen**, not the empty string: `## Either/or`
  gets `#either-or` (kramdown: `#eitheror`); `## I'm Lucky` gets `#i-m-lucky`
  (kramdown: `#im-lucky`).
- HTML inside a heading is ignored when computing the ID — only the text
  counts.
- Non-ASCII characters are dropped, not transliterated.
- Duplicate IDs get `-1`, `-2`, … suffixes.

You can always set an explicit ID with an inline attribute list:
<code>## My Heading {&#58; #my-id}</code>.

## Inline attribute lists

kramdown-style inline attribute lists (IALs) on headings —
<code>{&#58; .class #id key="value"}</code> — are supported; they are
rewritten to goldmark's attribute syntax before parsing.

> **Differs from Jekyll.** The IAL rewrite is a plain text substitution over
> the whole file, so a literal <code>{&#58; ...}</code> inside a code span or
> code block is also rewritten (the colon disappears). Not supported at all:
> **attribute list definitions** (ALDs, reusable
> <code>{&#58;refname: ...}</code> sets) and
> `markdown="span"` / `markdown="block"` on HTML elements —
> `markdown="1"` works.

## Syntax highlighting

Fenced code blocks and the `{% raw %}{% highlight %}{% endraw %}` tag are
highlighted by [chroma](https://github.com/alecthomas/chroma), with
Rouge-compatible CSS classes. Standard Liquid highlight blocks use Jekyll's
`figure.highlight > pre > code` shell and normalized language metadata.

Fenced blocks accept optional UI metadata after the language:

````markdown
```go title="main.go" frame="editor" startLineNumber=8 {2} ins={3-4} del="obsolete"
fmt.Println("ready")
return value
```
````

`title` is a nonempty double-quoted string. `frame` accepts `editor`, `terminal`,
or `none`. Line markers use 1-based inclusive selectors: `{2,4-6}`,
`ins={2,4-6}`, or `del={2,4-6}`. Text markers use exact, case-sensitive
strings: `"return value"`, `ins="added"`, or `del="removed"`. Text selectors
match every occurrence and may repeat. `showLineNumbers` is a bare flag;
`startLineNumber=N` enables numbering at decimal `N`, from 1 through 999999.
Inside quoted values, `\\` and `\"` are the only supported escapes.

Malformed recognized values, missing text, invalid or out-of-range lines,
overlapping insertion/deletion selections, duplicate numbering metadata, and
invalid numbering starts fail the build with the source path and line. Neutral
markers may overlap insertion/deletion markers; the latter presentation wins.
Unrecognized metadata retains its compatibility behavior.

A title without an explicit frame infers `terminal` for `bash`, `sh`, `shell`,
`console`, `powershell`, and `ps1`; other languages infer `editor`. Explicit
`frame` wins. Editor and terminal frames render as a semantic
`figure.highlight` with `data-code-frame`, an escaped `figcaption.code-title`
when titled, and the existing Chroma `pre > code` subtree. `frame="none"` and
annotation-only metadata retain the unframed wrapper.

Marked or numbered blocks add one `.code-line` span per logical source line.
Whole-line marker classes are `is-marked`, `is-inserted`, and `is-deleted`;
exact text matches use semantic `mark` elements with the same classes and one
accessible description per region. Chroma token spans are split only where
needed. Numbered lines carry sequential decimal `data-line-number` attributes,
and their `code` parent exposes `data-line-number-width` for a stable gutter;
numbers are never text inside `code`. Annotation markup therefore adds no
characters to selection or clipboard output. Blank source lines receive one
line span, and the structural trailing newline does not create another line.
Fences without recognized metadata retain their existing output byte-for-byte.
Code fences with an unrecognized language retain their plain `<pre><code>`
fallback.
