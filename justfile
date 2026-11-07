binary := "jigyll"
package := "github.com/reidransom/jigyll"
github_pages_image := "ghcr.io/github/pages-gem:v232@sha256:0ad87b5674ba06b23a86907a148953bdc0c98d37f626dd2387c20a0a692c5e58"

_version := `git describe --tags --exact-match 2>/dev/null || git rev-parse --short HEAD 2>/dev/null`
_build_date := `date +%FT%T%z`
_ldflags := "-X " + package + "/version.Version=" + _version + " -X " + package + "/version.BuildDate=" + _build_date

# list available recipes
_default:
    @just --list

# run tests
test:
    go test ./...

# install package dependencies and test tools
setup:
    go get -t ./...

# install pinned native benchmark tools, locked gems, and the default fixture
perf-setup:
    ./scripts/perf-setup

# list external direct imports
imports:
    go list -f '{{ "{{" }}join .Imports "\n"{{ "}}" }}' ./... | grep -v `go list -f '{{ "{{" }}.ImportPath{{ "}}" }}'` | grep '\.' | sort | uniq

# list external transitive dependencies
deps:
    go list -f '{{ "{{" }}join .Deps "\n"{{ "}}" }}' ./... | grep -v `go list -f '{{ "{{" }}.ImportPath{{ "}}" }}'` | grep '\.' | sort | uniq

# compile a race-detector build
race:
    go build -race -ldflags "{{_ldflags}}" -o {{binary}}-race {{package}}

# compile the binary
build:
    go mod tidy
    go build -ldflags "{{_ldflags}}" -o {{binary}} {{package}}

# serve the documentation site (docs/) with live reload
docs: build
    ./{{binary}} serve -s docs

# build the documentation site (docs/) into docs/_site
docs-build: build
    ./{{binary}} build -s docs

# build a site with the pinned GitHub Pages environment
pages-build repo destination="":
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ ! -d "{{repo}}" ]]; then
        echo "Site directory does not exist: {{repo}}" >&2
        exit 1
    fi
    repo_path="$(realpath "{{repo}}")"
    output="{{destination}}"
    if [[ -z "$output" ]]; then
        output="/tmp/${USER:-jigyll}/jigyll-compare/$(basename "$repo_path").github-pages"
    fi
    mkdir -p "$output"
    output="$(realpath "$output")"
    if [[ "$output" == "/" || "$output" == "$repo_path" ]]; then
        echo "Destination must not be the filesystem or site root: $output" >&2
        exit 1
    fi
    docker run --rm \
        -e JEKYLL_ENV=production \
        -e PAGES_REPO_NWO \
        -e JEKYLL_GITHUB_TOKEN \
        -v "$repo_path:/src/site" \
        -v "$output:/out" \
        "{{github_pages_image}}" \
        jekyll build --source /src/site --destination /out --trace
    echo "GitHub Pages output: $output"

# serve a site with the pinned GitHub Pages environment
pages-serve repo port="4000":
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ ! -d "{{repo}}" ]]; then
        echo "Site directory does not exist: {{repo}}" >&2
        exit 1
    fi
    if [[ ! "{{port}}" =~ ^[0-9]+$ ]]; then
        echo "Port must be numeric: {{port}}" >&2
        exit 1
    fi
    repo_path="$(realpath "{{repo}}")"
    docker run --rm --init \
        -e JEKYLL_ENV=production \
        -e PAGES_REPO_NWO \
        -e JEKYLL_GITHUB_TOKEN \
        -p "{{port}}:4000" \
        -v "$repo_path:/src/site" \
        "{{github_pages_image}}" \
        jekyll serve --source /src/site --destination /tmp/_site \
            --host 0.0.0.0 --port 4000 --force_polling --trace

# cross-compile for linux (amd64 + arm64)
buildlinux:
    mkdir -p dist
    GOOS=linux GOARCH=amd64 go build -ldflags "{{_ldflags}}" -o dist/{{binary}}-linux-amd64 {{package}}
    GOOS=linux GOARCH=arm64 go build -ldflags "{{_ldflags}}" -o dist/{{binary}}-linux-arm64 {{package}}

# bump patch version, tag, and push
release: lint test
    #!/usr/bin/env bash
    set -euo pipefail
    LATEST_TAG=$(git tag --sort=-v:refname | head -1)
    if [ -z "$LATEST_TAG" ]; then
        echo "No existing tags found. Creating v0.0.1" >&2
        NEW_TAG="v0.0.1"
    else
        echo "Latest tag: $LATEST_TAG"
        if [[ $LATEST_TAG =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
            MAJOR="${BASH_REMATCH[1]}"
            MINOR="${BASH_REMATCH[2]}"
            PATCH="${BASH_REMATCH[3]}"
            NEW_TAG="v${MAJOR}.${MINOR}.$((PATCH + 1))"
        else
            echo "Error: Could not parse version from tag: $LATEST_TAG" >&2
            exit 1
        fi
    fi
    echo "Creating new release: $NEW_TAG"
    git tag "$NEW_TAG"
    git push origin "$NEW_TAG"
    echo "Released $NEW_TAG"

# run linter
lint:
    golangci-lint run

# remove build artifacts
clean:
    rm -f {{binary}}
    rm -rf dist/

# install the binary
install:
    go install -ldflags "{{_ldflags}}" {{package}}
