package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, root, name, content string) string {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func lint(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestSourceCoverageAndRules(t *testing.T) {
	root := t.TempDir()
	put(t, root, "_config.yml", "url: https://example.org\nbaseurl: /blog\n")
	put(t, root, "index.md", "---\nimage: /images/hero.png\n---\n[portable]({{ '/ok' | relative_url }})\n[bad]({{ page.url }})\n[offsite](https://elsewhere.org/a)\n![photo](/images/p.png)\n[hard](https://example.org/blog/a)\n```md\n[example](/ignored)\n```\n[literal]({{ 'https://outside.org/x' | relative_url }})\n")
	put(t, root, "_data/links.yml", "links:\n  - /bad\ncanonical: https://example.org/blog/x\nimages:\n  - /images/p.png\n")
	put(t, root, "assets/site.css", "p { background: url('/images/x.png') }\n")
	put(t, root, "_includes/menu.html", "<a href='/bad'>Bad</a><form action='https://outside.org/submit'></form><img src='/assets/ok.png' srcset=','>\n<img src=\"{{ '/images/filtered.png' | relative_url }}\">\n")
	put(t, root, "scripts/app.js", `const a = "/bad";`)
	put(t, root, ".jigyll-source-lint.yml", "rules:\n  external-host: true\n  image-placement: true\n  javascript-url: true\nallowed_hosts: [elsewhere.org]\n")
	code, out, errout := lint(t, root)
	if code != 1 || errout != "" {
		t.Fatalf("exit %d stdout %q stderr %q", code, out, errout)
	}
	for _, want := range []string{
		"index.md:2: root-relative:", "index.md:2: image-placement:",
		"index.md:5: liquid-url:", "index.md:8: first-party-origin:",
		"index.md:12: external-host:", "_data/links.yml:5: image-placement:",
		"index.md:7: root-relative:", "_data/links.yml:2: root-relative:",
		"_data/links.yml:3: first-party-origin:",
		"assets/site.css:1: root-relative:", "assets/site.css:1: image-placement:",
		"_includes/menu.html:1: external-host:", "scripts/app.js:1: javascript-url:",
		"_includes/menu.html:2: image-placement:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in %s", want, out)
		}
	}
	for _, unwanted := range []string{"index.md:4:", "index.md:6:", "index.md:10:", "_includes/menu.html:1: image-placement:"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("unexpected %s in %s", unwanted, out)
		}
	}
}

func TestSharedSiteFileAndLinePrecedence(t *testing.T) {
	root := t.TempDir()
	put(t, root, "_config.yml", "baseurl: /site\n")
	put(t, root, "pages/a.md", "[one](/a)\n[two](/b)\n")
	put(t, root, "pages/b.md", "[three](/c)\n")
	shared := put(t, t.TempDir(), "shared.yml", "rules:\n  root-relative: false\nfiles:\n  - glob: 'pages/*.md'\n    rules: {root-relative: true}\nlines:\n  - file: pages/a.md\n    line: 1\n    rules: {root-relative: true}\n")
	put(t, root, ".jigyll-source-lint.yml", "rules: {root-relative: true}\nfiles:\n  - glob: 'pages/*.md'\n    rules: {root-relative: false}\nlines:\n  - file: pages/a.md\n    line: 2\n    rules: {root-relative: true}\n")
	code, out, errout := lint(t, "--policy", shared, root)
	if code != 1 || errout != "" || !strings.Contains(out, "pages/a.md:1: root-relative:") || !strings.Contains(out, "pages/a.md:2: root-relative:") || strings.Contains(out, "pages/b.md:") {
		t.Fatalf("exit %d stdout %q stderr %q", code, out, errout)
	}
}

func TestNoPoliciesAndBadConfiguration(t *testing.T) {
	root := t.TempDir()
	put(t, root, "_config.yml", "url: https://example.org\n")
	put(t, root, "index.md", "[root](/fine) [external](https://other.org)\n")
	code, out, errout := lint(t, root)
	if code != 0 || out != "" || errout != "" {
		t.Fatalf("exit %d stdout %q stderr %q", code, out, errout)
	}
	bad := put(t, root, ".jigyll-source-lint.yml", "rules: {invented-rule: true}\n")
	code, _, errout = lint(t, "--site-policy", bad, root)
	if code != 2 || !strings.Contains(errout, "unknown rule") {
		t.Fatalf("bad policy exit %d stderr %q", code, errout)
	}
}
