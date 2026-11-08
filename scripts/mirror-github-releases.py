#!/usr/bin/env python3
"""Mirror published Forgejo release notes and assets to the GitHub mirror."""

import argparse
import hashlib
import json
import os
import sys
import tempfile
import urllib.error
import urllib.parse
import urllib.request

SOURCE = "https://src.r2ware.dev/api/v1/repos/reid/jigyll"
TARGET = "https://api.github.com/repos/reidransom/jigyll"


def request(url, token, *, method="GET", data=None, headers=None, missing_ok=False):
    request_headers = {"Accept": "application/json", "User-Agent": "jigyll-release-mirror"}
    if token:
        request_headers["Authorization"] = f"token {token}"
    if headers:
        request_headers.update(headers)
    if isinstance(data, dict):
        data = json.dumps(data).encode()
        request_headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=request_headers, method=method)
    try:
        return urllib.request.urlopen(req, timeout=120)
    except urllib.error.HTTPError as error:
        if missing_ok and error.code == 404:
            return None
        raise RuntimeError(f"{method} {url}: HTTP {error.code}") from error


def api(url, token, **kwargs):
    response = request(url, token, **kwargs)
    if response is None:
        return None
    with response:
        body = response.read()
        return json.loads(body) if body else None


def pages(url, token):
    page = 1
    while True:
        items = api(f"{url}?page={page}&limit=50&per_page=50", token)
        yield from items
        if len(items) < 50:
            return
        page += 1


def release_url(base, tag):
    return f"{base}/releases/tags/{urllib.parse.quote(tag, safe='')}"


def sync_assets(source, target, source_token, target_token):
    destination = list(pages(f"{TARGET}/releases/{target['id']}/assets", target_token))
    by_name = {asset["name"]: asset for asset in destination}
    source_names = {asset["name"] for asset in source["assets"]}
    for asset in source["assets"]:
        # Hash the source even when sizes match: replacing an asset need not change its size.
        digest = hashlib.sha256()
        with tempfile.TemporaryFile() as downloaded:
            with request(asset["browser_download_url"], source_token) as response:
                while chunk := response.read(1024 * 1024):
                    digest.update(chunk)
                    downloaded.write(chunk)
            size = downloaded.tell()
            if size != asset["size"]:
                raise RuntimeError(f"Incomplete source asset: {asset['name']}")
            existing = by_name.get(asset["name"])
            expected_digest = f"sha256:{digest.hexdigest()}"
            if existing and existing.get("digest") == expected_digest:
                print(f"  unchanged {asset['name']}")
                continue
            if existing:
                api(f"{TARGET}/releases/assets/{existing['id']}", target_token, method="DELETE")
            downloaded.seek(0)
            upload_url = target["upload_url"].split("{", 1)[0]
            upload_url += "?" + urllib.parse.urlencode({"name": asset["name"]})
            uploaded = api(
                upload_url,
                target_token,
                method="POST",
                data=downloaded,
                headers={"Content-Type": "application/octet-stream", "Content-Length": str(size)},
            )
            if uploaded.get("digest") != expected_digest:
                raise RuntimeError(f"Uploaded asset checksum mismatch: {asset['name']}")
            print(f"  uploaded {asset['name']}")
    # Only prune assets on releases that exist in Forgejo; retain historical GitHub releases.
    for asset in destination:
        if asset["name"] not in source_names:
            api(f"{TARGET}/releases/assets/{asset['id']}", target_token, method="DELETE")
            print(f"  removed stale asset {asset['name']}")


def sync_release(source, source_token, target_token):
    if source["draft"]:
        print(f"Skipping draft {source['tag_name']}")
        return
    tag = source["tag_name"]
    # The native Forgejo push mirror owns tags. Never create a tag from GitHub's main tip.
    ref = api(
        f"{TARGET}/git/ref/tags/{urllib.parse.quote(tag, safe='')}",
        target_token,
        missing_ok=True,
    )
    if ref is None:
        raise RuntimeError(f"Tag {tag} has not reached GitHub; synchronize the Forgejo push mirror first")
    target = api(release_url(TARGET, tag), target_token, missing_ok=True)
    fields = {"tag_name": tag, "name": source["name"], "body": source["body"] or "", "draft": False, "prerelease": source["prerelease"]}
    if target is None:
        target = api(f"{TARGET}/releases", target_token, method="POST", data=fields)
        print(f"Created release {tag}")
    elif any(target.get(key) != value for key, value in fields.items()):
        target = api(f"{TARGET}/releases/{target['id']}", target_token, method="PATCH", data=fields)
        print(f"Updated release {tag}")
    else:
        print(f"Release {tag} metadata unchanged")
    sync_assets(source, target, source_token, target_token)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tag", help="Synchronize one release instead of all published releases")
    parser.add_argument("--delete", action="store_true", help="Remove a deleted Forgejo release from GitHub (requires --tag)")
    args = parser.parse_args()
    if args.delete and not args.tag:
        parser.error("--delete requires --tag")
    source_token = os.environ.get("GITEA_TOKEN", "")
    target_token = os.environ.get("MIRROR_GITHUB_TOKEN", "")
    if not target_token:
        parser.error("MIRROR_GITHUB_TOKEN is required")
    if args.tag:
        source = api(release_url(SOURCE, args.tag), source_token, missing_ok=args.delete)
        if args.delete:
            if source is not None:
                raise RuntimeError(f"Refusing to delete {args.tag}: release still exists in Forgejo")
            target = api(release_url(TARGET, args.tag), target_token, missing_ok=True)
            if target:
                api(f"{TARGET}/releases/{target['id']}", target_token, method="DELETE")
                print(f"Removed release {args.tag}")
        else:
            sync_release(source, source_token, target_token)
    else:
        for source in pages(f"{SOURCE}/releases", source_token):
            sync_release(source, source_token, target_token)


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, urllib.error.URLError) as error:
        print(f"Release mirror failed: {error}", file=sys.stderr)
        sys.exit(1)
