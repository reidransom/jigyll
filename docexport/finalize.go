package docexport

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

// Finalize refreshes a built documentation snapshot after asset publication
// rewrites. All inputs are read through os.Root, which confines symlink traversal
// as well as lexical paths. No source tree, configuration, or network is used.
// Validation and staging finish before any artifact is replaced. Each replacement
// is atomic, but the set is not a filesystem transaction: the caller must publish
// the output directory only after this function succeeds.
func Finalize(directory string, assetMap map[string]string) error {
	for original, revised := range assetMap {
		target, err := url.Parse(revised)
		if original == "" || strings.TrimSpace(revised) == "" || err != nil || target.User != nil || (target.Scheme != "" && target.Scheme != "http" && target.Scheme != "https") || strings.ContainsAny(revised, "\\\x00\r\n") {
			return fmt.Errorf("invalid asset revision mapping %q to %q", original, revised)
		}
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return fmt.Errorf("open documentation output: %w", err)
	}
	defer root.Close()
	data, err := readSnapshotFile(root, "docs/manifest.json")
	if err != nil {
		return err
	}
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return fmt.Errorf("decode docs/manifest.json: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("decode docs/manifest.json: expected exactly one JSON object")
	}
	markdown := make(map[string][]byte, len(manifest.Articles))
	for _, article := range manifest.Articles {
		if err := validateArticle(article); err != nil {
			return err
		}
		route := article.URL + "index.md"
		source, err := readSnapshotFile(root, strings.TrimPrefix(route, "/"))
		if err != nil {
			return err
		}
		markdown[route] = source
	}
	corpus := Corpus{SchemaVersion: SchemaVersion, Records: []Section{}}
	if err := validateSnapshot(manifest, corpus, markdown); err != nil {
		return err
	}
	for i := range manifest.Articles {
		article := &manifest.Articles[i]
		route := article.URL + "index.md"
		revised, err := Markdown(markdown[route], article.CanonicalURL, assetMap)
		if err != nil {
			return fmt.Errorf("article %q Markdown: %w", article.ID, err)
		}
		markdown[route] = revised
		page, err := readSnapshotFile(root, strings.TrimPrefix(article.URL+"index.html", "/"))
		if err != nil {
			return err
		}
		body, err := finalArticleBody(page)
		if err != nil {
			return fmt.Errorf("article %q: %w", article.ID, err)
		}
		records, headings, err := Sections(*article, body)
		if err != nil {
			return err
		}
		article.Sections = headings
		corpus.Records = append(corpus.Records, records...)
	}
	artifacts, err := Assemble(manifest, corpus, markdown)
	if err != nil {
		return err
	}
	return replaceSnapshotFiles(root, artifacts)
}

func readSnapshotFile(root *os.Root, name string) ([]byte, error) {
	file, err := root.Open(name)
	if err != nil {
		return nil, fmt.Errorf("read documentation %s: %w", name, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat documentation %s: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("documentation %s is not a regular file", name)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read documentation %s: %w", name, err)
	}
	return data, nil
}

func finalArticleBody(page []byte) (string, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return "", fmt.Errorf("parse final HTML: %w", err)
	}
	var articles []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "article" {
			for _, class := range strings.Fields(sectionAttribute(node, "class")) {
				if class == "docs-article" {
					articles = append(articles, node)
					break
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if len(articles) != 1 {
		return "", fmt.Errorf("final HTML must contain exactly one article.docs-article (found %d)", len(articles))
	}
	var body strings.Builder
	for child := articles[0].FirstChild; child != nil; child = child.NextSibling {
		if err := html.Render(&body, child); err != nil {
			return "", fmt.Errorf("serialize final article body: %w", err)
		}
	}
	return body.String(), nil
}

func replaceSnapshotFiles(root *os.Root, artifacts map[string][]byte) error {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return fmt.Errorf("stage documentation snapshot: %w", err)
	}
	prefix := ".jigyll-docs-" + hex.EncodeToString(nonce[:])
	// All destinations already belong to the initial snapshot. Reject missing,
	// nonregular, or symlink destinations before creating any staging files.
	for route := range artifacts {
		info, err := root.Lstat(strings.TrimPrefix(route, "/"))
		if err != nil {
			return fmt.Errorf("validate documentation destination %s: %w", route, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("documentation destination %s is not a regular file", route)
		}
	}
	routes := make([]string, 0, len(artifacts))
	for route := range artifacts {
		routes = append(routes, route)
	}
	sort.Strings(routes)
	staged := make([]string, 0, len(routes))
	defer func() {
		for _, name := range staged {
			_ = root.Remove(name)
		}
	}()
	for i, route := range routes {
		name := fmt.Sprintf("%s-%d", prefix, i)
		file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return fmt.Errorf("stage documentation %s: %w", route, err)
		}
		staged = append(staged, name)
		_, writeErr := file.Write(artifacts[route])
		closeErr := file.Close()
		if writeErr != nil {
			return fmt.Errorf("stage documentation %s: %w", route, writeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close staged documentation %s: %w", route, closeErr)
		}
	}
	for i, route := range routes {
		if err := root.Rename(staged[i], strings.TrimPrefix(route, "/")); err != nil {
			return fmt.Errorf("replace documentation %s (output must not be published): %w", route, err)
		}
	}
	return nil
}
