binary := "gojekyll"
package := "github.com/osteele/gojekyll"

_version := `git describe --tags --exact-match 2>/dev/null || git rev-parse --short HEAD 2>/dev/null`
_build_date := `date +%FT%T%z`
_ldflags := "-X " + package + "/version.Version=" + _version + " -X " + package + "/version.BuildDate=" + _build_date

# list available recipes
_default:
    @just --list

# run tests
test:
    go test ./...

# compile the binary
build:
    go mod tidy
    go build -ldflags "{{_ldflags}}" -o {{binary}} {{package}}

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
