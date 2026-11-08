#!/usr/bin/env python3
"""Publish GoReleaser's generated manifests to the existing GitHub packages."""

import base64
import json
import os
from pathlib import Path
import sys
from urllib.error import HTTPError
from urllib.request import Request, urlopen


PACKAGES = (
    (
        "reidransom/homebrew-tap",
        "Formula/jigyll.rb",
        "dist/homebrew/Formula/jigyll.rb",
        "HOMEBREW_TAP_GITHUB_TOKEN",
    ),
    (
        "reidransom/scoop-bucket",
        "bucket/jigyll.json",
        "dist/scoop/bucket/jigyll.json",
        "SCOOP_BUCKET_GITHUB_TOKEN",
    ),
)


def request(method, url, token, payload=None):
    data = None if payload is None else json.dumps(payload).encode()
    headers = {
        "Accept": "application/vnd.github+json",
        "Authorization": f"Bearer {token}",
        "X-GitHub-Api-Version": "2022-11-28",
        "User-Agent": "jigyll-package-publisher",
    }
    if data is not None:
        headers["Content-Type"] = "application/json"
    with urlopen(Request(url, data=data, headers=headers, method=method), timeout=60) as response:
        return json.load(response)


def main():
    # Resolve both prerequisites before publishing either package.
    packages = []
    for repository, target, source, secret in PACKAGES:
        token = os.environ.get(secret)
        if not token:
            raise ValueError(f"{secret} is required")
        packages.append((repository, target, Path(source).read_bytes(), token))

    for repository, target, content, token in packages:
        url = f"https://api.github.com/repos/{repository}/contents/{target}"
        try:
            existing = request("GET", url + "?ref=main", token)
        except HTTPError as error:
            if error.code != 404:
                raise
            existing = None
        if existing and base64.b64decode(existing["content"]) == content:
            print(f"{repository}/{target} is already current")
            continue
        payload = {
            "message": "Update jigyll package manifest",
            "branch": "main",
            "content": base64.b64encode(content).decode("ascii"),
            "committer": {"name": "goreleaserbot", "email": "bot@goreleaser.com"},
        }
        if existing:
            payload["sha"] = existing["sha"]
        request("PUT", url, token, payload)
        print(f"Published {repository}/{target}")


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, KeyError) as error:
        print(f"Package publication failed: {error}", file=sys.stderr)
        sys.exit(1)
