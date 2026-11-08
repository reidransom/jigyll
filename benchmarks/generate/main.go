// Command generate creates a deterministic, network-free full-build benchmark site.
// Adapted from osteele/gojekyll, benchmarks/generate/main.go at
// 4d08286474a1d218387808d7d7c695b2cc01843a.
// The nested layouts, includes, and content retain the upstream workload;
// no performance or template-cache bottleneck is assumed.
//
// # MIT License
//
// # Copyright (c) 2017 Oliver Steele
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

const (
	defaultPosts      = 2000
	defaultPages      = 20
	generatorRevision = "2"
	upstreamRevision  = "4d08286474a1d218387808d7d7c695b2cc01843a"
	manifestPath      = "_benchmark/manifest.json"
)

var firstPostDate = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
var categories = []string{"tech", "science", "travel", "food", "music"}
var tags = []string{"go", "ruby", "python", "javascript", "rust", "benchmarks", "performance", "caching"}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	posts := flags.Int("posts", defaultPosts, "number of posts")
	pages := flags.Int("pages", defaultPages, "number of standalone pages (plus index)")
	output := flags.String("output", "", "new site directory inside a private harness workspace; parent must exist")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *output == "" {
		return fmt.Errorf("provide -output with a new directory in a private harness workspace; no positional arguments")
	}
	if err := generate(*output, *posts, *pages); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Generated %d posts, %d standalone pages, index, 10 includes, and 4 layouts in %s\n", *posts, *pages, *output)
	fmt.Fprintf(stdout, "Future-post policy: future: true (all requested posts); timezone: Etc/UTC; incremental: false\nManifest: %s\n", filepath.Join(*output, manifestPath))
	return nil
}

// Manifest is deterministic JSON, excluded from site output by its underscore
// directory. Paths are slash-separated and relative to source/destination roots;
// URLs are site-root relative. Every output must exist. Preflight should check
// each output's title, text, and links plus the literal HTML fragments for its
// kind in Checks. Inputs hash all source files except the manifest itself.
// Schema and generator revisions change when their contracts/content change.
type Manifest struct {
	SchemaVersion     int                 `json:"schema_version"`
	GeneratorRevision string              `json:"generator_revision"`
	UpstreamRevision  string              `json:"upstream_revision"`
	Parameters        Parameters          `json:"parameters"`
	Future            bool                `json:"future"`
	Timezone          string              `json:"timezone"`
	Incremental       bool                `json:"incremental"`
	Inputs            []Input             `json:"inputs"`
	Outputs           []Output            `json:"outputs"`
	Checks            map[string][]string `json:"checks"`
}

type Parameters struct {
	Posts    int `json:"posts"`
	Pages    int `json:"pages"`
	Includes int `json:"includes"`
	Layouts  int `json:"layouts"`
}

type Input struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Output struct {
	Kind   string   `json:"kind"`
	Source string   `json:"source"`
	URL    string   `json:"url"`
	Path   string   `json:"path"`
	Title  string   `json:"title"`
	Text   []string `json:"text"`
	Links  []string `json:"links"`
}

// generate claims a new directory atomically. It never replaces, removes, or
// reuses an existing directory (even an empty directory or symlink). On failure
// the newly created partial fixture is retained for diagnostics, not deleted.
func generate(dir string, posts, pages int) error {
	if posts < 0 || pages < 0 {
		return fmt.Errorf("posts and pages must be nonnegative")
	}
	// Keep dates in the four-digit ISO range accepted by both engines. Check
	// before allocation and before AddDate, including for overflowing arguments.
	maxPosts := int((time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC).Unix()-firstPostDate.Unix())/86400) + 1
	if posts > maxPosts {
		return fmt.Errorf("posts must be at most %d (last supported date is 9999-12-31)", maxPosts)
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return fmt.Errorf("allocate new fixture directory (existing paths are never replaced): %w", err)
	}
	if err := writeFixtureFiles(dir); err != nil {
		return err
	}
	manifest := newManifest(posts, pages)
	if err := writeFixtureContent(dir, posts, pages, &manifest); err != nil {
		return err
	}
	if err := collectFixtureInputs(dir, &manifest); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, manifestPath), string(data)+"\n")
}

func writeFixtureFiles(dir string) error {
	for _, subdir := range []string{"_layouts", "_includes", "_posts", "_benchmark"} {
		if err := os.Mkdir(filepath.Join(dir, subdir), 0o700); err != nil {
			return err
		}
	}
	if err := writeFile(filepath.Join(dir, "_config.yml"), configYML); err != nil {
		return err
	}
	for subdir, files := range map[string]map[string]string{"_layouts": layouts, "_includes": includes} {
		for name, content := range files {
			if err := writeFile(filepath.Join(dir, subdir, name), content); err != nil {
				return err
			}
		}
	}
	return writeFile(filepath.Join(dir, "index.html"), indexHTML)
}

func writeFixtureContent(dir string, posts, pages int, manifest *Manifest) error {
	postTmpl := template.Must(template.New("post").Funcs(template.FuncMap{"title": titleCase}).Parse(postTemplate))
	for i := range posts {
		post := postAt(i)
		if err := writeTemplate(filepath.Join(dir, manifest.Outputs[i].Source), postTmpl, post); err != nil {
			return err
		}
	}
	pageTmpl := template.Must(template.New("page").Parse(pageTemplate))
	for i := range pages {
		if err := writeTemplate(filepath.Join(dir, fmt.Sprintf("page-%03d.md", i)), pageTmpl, i); err != nil {
			return err
		}
	}
	return nil
}

func collectFixtureInputs(dir string, manifest *Manifest) error {
	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(content)
		manifest.Inputs = append(manifest.Inputs, Input{filepath.ToSlash(rel), hex.EncodeToString(hash[:])})
		return nil
	})
}

func newManifest(posts, pages int) Manifest {
	common := []string{"<!DOCTYPE html>", "<header>", "<main>", "<footer>", `<aside class="sidebar">`, `<nav class="breadcrumbs">`, "<article>"}
	postChecks := append(append([]string{}, common...), `<div class="post-content">`, `<div class="post-meta">`, `<div class="tags">`, `<div class="social-share">`, "<strong>bold</strong>", "<code>code</code>")
	if posts > 1 {
		postChecks = append(postChecks, `<div class="related-posts">`)
	}
	m := Manifest{
		SchemaVersion: 1, GeneratorRevision: generatorRevision, UpstreamRevision: upstreamRevision,
		Parameters: Parameters{posts, pages, len(includes), len(layouts)}, Future: true, Timezone: "Etc/UTC",
		Inputs: []Input{}, Outputs: make([]Output, 0, posts+pages+1),
		Checks: map[string][]string{
			"post":  postChecks,
			"page":  append(append([]string{}, common...), `<div class="page-content">`, `<nav class="toc">`),
			"index": common,
		},
	}
	links := []string{"/"}
	if pages > 0 {
		links = append(links, "/page-000.html")
	}
	for i := range posts {
		post := postAt(i)
		date := firstPostDate.AddDate(0, 0, i)
		url := fmt.Sprintf("/%s/%s/post-%04d/", post.Category, date.Format("2006/01/02"), i)
		m.Outputs = append(m.Outputs, Output{
			Kind: "post", Source: fmt.Sprintf("_posts/%s-post-%04d.md", post.Date, i), URL: url, Path: strings.TrimPrefix(url, "/") + "index.html",
			Title: fmt.Sprintf("Post Number %d: Exploring %s Topics", i, titleCase(post.Category)),
			Text:  []string{fmt.Sprintf("Introduction to Post %d", i), fmt.Sprintf("Post %d is part of the %s category", i, post.Category)}, Links: links,
		})
	}
	for i := range pages {
		path := fmt.Sprintf("page-%03d.html", i)
		m.Outputs = append(m.Outputs, Output{Kind: "page", Source: fmt.Sprintf("page-%03d.md", i), URL: "/" + path, Path: path,
			Title: fmt.Sprintf("Page %d", i), Text: []string{fmt.Sprintf("This is standalone page number %d.", i)}, Links: append(append([]string{}, links...), "#first-section", "#second-section")})
	}
	indexLinks := append([]string{}, links...)
	if posts > 0 {
		indexLinks = append(indexLinks, m.Outputs[0].URL, m.Outputs[posts-1].URL)
	}
	m.Outputs = append(m.Outputs, Output{Kind: "index", Source: "index.html", URL: "/", Path: "index.html", Title: "Home", Text: []string{"Benchmark Site", "Posts"}, Links: indexLinks})
	return m
}

type postData struct {
	N        int
	Date     string
	Category string
	Tags     []string
}

func postAt(i int) postData {
	return postData{i, firstPostDate.AddDate(0, 0, i).Format("2006-01-02"), categories[i%len(categories)], []string{tags[i%len(tags)], tags[(i+3)%len(tags)]}}
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

func writeTemplate(path string, tmpl *template.Template, data any) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := tmpl.Execute(file, data); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// ---------- Static content ----------

const configYML = `title: Benchmark Site
description: A generated site for full-build benchmarks
future: true
timezone: Etc/UTC
incremental: false
url: "http://example.com"
baseurl: ""
permalink: /:categories/:year/:month/:day/:title/

defaults:
  - scope:
      path: ""
      type: "posts"
    values:
      layout: "post"
  - scope:
      path: ""
    values:
      layout: "page"
`

var layouts = map[string]string{
	"base.html": `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>{{ page.title | default: site.title }}</title>
</head>
<body>
{% include header.html %}
<main>
  {{ content }}
</main>
{% include footer.html %}
</body>
</html>
`,
	"default.html": `---
layout: base
---
{% include sidebar.html %}
{% include breadcrumbs.html %}
<article>
  {{ content }}
</article>
`,
	"post.html": `---
layout: default
---
{% include post-meta.html %}
<div class="post-content">
  {{ content }}
</div>
{% include tag-list.html %}
{% include related-posts.html %}
{% include social-share.html %}
{% include pagination.html %}
`,
	"page.html": `---
layout: default
---
{% include toc.html %}
<div class="page-content">
  {{ content }}
</div>
`,
}

var includes = map[string]string{
	"header.html": `<header>
  <nav>
    <a href="/">{{ site.title }}</a>
    <ul>
    {% for p in site.pages %}
      {% if p.title %}
      <li><a href="{{ p.url }}">{{ p.title | escape }}</a></li>
      {% endif %}
    {% endfor %}
    </ul>
  </nav>
</header>
`,
	"footer.html": `<footer>
  <p>&copy; 2020 {{ site.title | escape }}</p>
  <p>{{ site.description | escape }}</p>
  <p>Benchmark fixture adapted from <a href="https://github.com/osteele/gojekyll">gojekyll</a></p>
</footer>
`,
	"sidebar.html": `<aside class="sidebar">
  <h3>Recent Posts</h3>
  <ul>
  {% for post in site.posts limit:10 %}
    <li>
      <a href="{{ post.url | relative_url }}">{{ post.title | escape }}</a>
      <span>{{ post.date | date: "%b %-d, %Y" }}</span>
    </li>
  {% endfor %}
  </ul>
</aside>
`,
	"tag-list.html": `{% if page.tags.size > 0 %}
<div class="tags">
  <span>Tags:</span>
  {% for tag in page.tags %}
    <a href="/tags/{{ tag | slugify }}/">{{ tag | capitalize }}</a>
    {% unless forloop.last %}, {% endunless %}
  {% endfor %}
</div>
{% endif %}
`,
	"post-meta.html": `<div class="post-meta">
  <time datetime="{{ page.date | date_to_xmlschema }}">
    {{ page.date | date: "%B %-d, %Y" }}
  </time>
  {% if page.categories.size > 0 %}
  <span class="categories">
    in {% for cat in page.categories %}
      <a href="/categories/{{ cat | slugify }}/">{{ cat | capitalize }}</a>
      {% unless forloop.last %}, {% endunless %}
    {% endfor %}
  </span>
  {% endif %}
</div>
`,
	"pagination.html": `{% if paginator %}
<nav class="pagination">
  {% if paginator.previous_page %}
    <a href="{{ paginator.previous_page_path | relative_url }}">&laquo; Newer</a>
  {% else %}
    <span class="disabled">&laquo; Newer</span>
  {% endif %}
  <span>Page {{ paginator.page }} of {{ paginator.total_pages }}</span>
  {% if paginator.next_page %}
    <a href="{{ paginator.next_page_path | relative_url }}">Older &raquo;</a>
  {% else %}
    <span class="disabled">Older &raquo;</span>
  {% endif %}
</nav>
{% endif %}
`,
	"breadcrumbs.html": `{% assign crumbs = page.url | split: "/" %}
<nav class="breadcrumbs">
  <a href="/">Home</a>
  {% for crumb in crumbs %}
    {% if crumb != "" %}
      <span>/</span>
      <a href="/{{ crumb }}/">{{ crumb | replace: "-", " " | capitalize }}</a>
    {% endif %}
  {% endfor %}
</nav>
`,
	"related-posts.html": `{% if site.posts.size > 1 %}
<div class="related-posts">
  <h3>Related Posts</h3>
  <ul>
  {% for post in site.related_posts limit:5 %}
    <li>
      <a href="{{ post.url | relative_url }}">{{ post.title | escape }}</a>
      <span>{{ post.date | date: "%b %-d, %Y" }}</span>
    </li>
  {% endfor %}
  </ul>
</div>
{% endif %}
`,
	"social-share.html": `<div class="social-share">
  {% if page.title %}
    {% assign share_title = page.title | url_encode %}
  {% else %}
    {% assign share_title = site.title | url_encode %}
  {% endif %}
  {% assign share_url = page.url | absolute_url | url_encode %}
  {% unless page.hide_share %}
    <a href="https://twitter.com/intent/tweet?text={{ share_title }}&url={{ share_url }}">Twitter</a>
  {% endunless %}
</div>
`,
	"toc.html": `{% if page.toc %}
<nav class="toc">
  <h3>Table of Contents</h3>
  <ul>
  {% for section in page.toc %}
    <li>
      <a href="#{{ section.title | slugify }}">{{ section.title | escape }}</a>
      {% if section.children %}
      <ul>
        {% for child in section.children %}
        <li><a href="#{{ child.title | slugify }}">{{ child.title | escape }}</a></li>
        {% endfor %}
      </ul>
      {% endif %}
    </li>
  {% endfor %}
  </ul>
</nav>
{% endif %}
`,
}

const indexHTML = `---
layout: default
title: Home
---
<h1>{{ site.title }}</h1>
<p>{{ site.description }}</p>

<h2>Posts</h2>
<ul>
{% for post in site.posts %}
  <li>
    <a href="{{ post.url | relative_url }}">{{ post.title | escape }}</a>
    <span>{{ post.date | date: "%b %-d, %Y" }}</span>
    {% if post.categories.size > 0 %}
      <span>in {{ post.categories | join: ", " }}</span>
    {% endif %}
  </li>
{% endfor %}
</ul>
`

const postTemplate = `---
title: "Post Number {{ .N }}: Exploring {{ .Category | title }} Topics"
date: {{ .Date }}
categories: [{{ .Category }}]
tags: [{{ index .Tags 0 }}, {{ index .Tags 1 }}]
excerpt: "This is post number {{ .N }} about {{ .Category }}."
---

## Introduction to Post {{ .N }}

This is a benchmark post about **{{ .Category }}**. It exists to exercise
the Liquid template engine across layouts and includes.

### Section One

Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod
tempor incididunt ut labore et dolore magna aliqua.

{{ "{{ page.title }}" }}  was published on {{ "{{ page.date | date: \"%B %-d, %Y\" }}" }}.

### Section Two

{{ "{% for tag in page.tags %}{{ tag | capitalize }}{% unless forloop.last %}, {% endunless %}{% endfor %}" }}

More content to give the Markdown renderer something to chew on.
Paragraph with **bold**, *italic*, and ` + "`code`" + ` formatting.

- List item one
- List item two
- List item three

> A blockquote to add variety to the rendered output.

### Conclusion

Post {{ .N }} is part of the {{ .Category }} category and is tagged with
{{ index .Tags 0 }} and {{ index .Tags 1 }}.
`

const pageTemplate = `---
layout: page
title: "Page {{ . }}"
permalink: /page-{{ printf "%03d" . }}.html
toc:
  - title: First Section
  - title: Second Section
---

## Page {{ . }}

This is standalone page number {{ . }}. It uses the page layout,
which includes the table-of-contents include.

### First Section

Content for the first section of page {{ . }}.

### Second Section

More content to exercise the template engine with page layout rendering.
`
