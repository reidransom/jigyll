#!/bin/bash -e

# Docker images are published by .forgejo/workflows/docker.yml on v* tags.
# Requires GITEA_TOKEN plus the dedicated Homebrew and Scoop GitHub tokens.
goreleaser release
python3 scripts/publish-package-manifests.py