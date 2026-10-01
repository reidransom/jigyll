// jigyll-lint audits one Jigyll site without running the generator.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/reidransom/jigyll/sourceportability"
)

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("jigyll-lint", flag.ContinueOnError)
	flags.SetOutput(stderr)
	shared := flags.String("policy", "", "shared YAML policy used across sites")
	sitePolicy := flags.String("site-policy", "", "site policy (default: .jigyll-source-lint.yml if present)")
	config := flags.String("config", "", "site configuration (default: _config.yml)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: jigyll-lint [--policy shared.yml] [--site-policy site.yml] [--config _config.yml] SITE_ROOT")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	root, err := filepath.Abs(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	local := *sitePolicy
	if local == "" {
		candidate := filepath.Join(root, ".jigyll-source-lint.yml")
		if _, err := os.Stat(candidate); err == nil {
			local = candidate
		} else if !os.IsNotExist(err) {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	findings, err := sourceportability.Scan(sourceportability.Options{
		Site: root, SharedPolicy: *shared, SitePolicy: local, SiteConfig: *config,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	for _, finding := range findings {
		fmt.Fprintln(stdout, finding)
	}
	if len(findings) != 0 {
		return 1
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
