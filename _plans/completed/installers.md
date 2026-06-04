Yes. The main conventions people use for Go CLIs today are:

- `go install module/path/cmd@version` for Go-savvy users
- A single-line `curl | bash` / `curl | sh` installer that drops a binary into `~/bin` or `/usr/local/bin`
- OS package managers (Homebrew, apt, etc.) generated with tools like Goreleaser

Below are the common patterns and how you might pick one.

## 1. Lean on `go install` when possible

For users who have Go installed, the idiomatic convention is:

```bash
go install github.com/you/yourtool/cmd/yourtool@latest
```

This:

- Clones the repo, compiles, and places the binary in `GOBIN` (or `GOPATH/bin`) automatically. [reddit](https://www.reddit.com/r/golang/comments/1cr84fj/how_to_distribute_a_cli_tool_with_go_install/)
- Requires zero manual PATH editing if the user already has Go set up correctly. [digitalocean](https://www.digitalocean.com/community/tutorials/how-to-build-and-install-go-programs)

You can surface this as your “developer install” path and not worry about packaging for those users.

Example you can put in your README:

```bash
# For users with Go 1.17+
go install github.com/r2ware/yourtool/cmd/yourtool@latest
```

## 2. “curl | sh” style installer script

For non-Go users on Unix-like systems, the de facto convention is a one-liner that fetches an installer script and runs it:

```bash
curl -fsSL https://get.yoursite.dev/install.sh | sh
# or
bash <(curl -fsSL https://get.yoursite.dev/install.sh)
```

You see this pattern with things like language installers and even Go itself via community scripts. [github](https://github.com/kerolloz/go-installer)

Your `install.sh` typically:

- Detects OS and architecture (`uname -s`, `uname -m`)  
- Resolves the latest release URL from GitHub Releases / S3 / your host  
- Downloads the correct tarball or binary  
- Installs it into a directory on `PATH` (e.g. `/usr/local/bin` with sudo, or `~/.local/bin`/`~/bin` if you want to avoid sudo)  
- Optionally prints a note if it had to create a bin dir that is not yet on `PATH`

Conceptually:

```bash
#!/usr/bin/env bash
set -euo pipefail

# detect OS/arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# figure out latest version (hardcode or hit GitHub API)
VERSION="v0.1.0"

TAR_URL="https://github.com/you/yourtool/releases/download/${VERSION}/yourtool_${OS}_${ARCH}.tar.gz"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

curl -fsSL "$TAR_URL" -o "$TMPDIR/yourtool.tar.gz"
tar -C "$TMPDIR" -xzf "$TMPDIR/yourtool.tar.gz"

INSTALL_DIR="${HOME}/.local/bin"   # or /usr/local/bin with sudo
mkdir -p "$INSTALL_DIR"
mv "$TMPDIR/yourtool" "$INSTALL_DIR/yourtool"

echo "Installed to $INSTALL_DIR/yourtool"
```

You keep your release artifacts on GitHub Releases, which is a common pattern for Go binaries. [ibilalkayy.hashnode](https://ibilalkayy.hashnode.dev/creating-and-distributing-cli-applications-with-golang-and-github)

Security note: people are increasingly wary of piping curl to shell; some projects instead recommend:

```bash
curl -fsSL https://get.yoursite.dev/install.sh -o install.sh
sh install.sh
```

But the underlying distribution convention is the same. [chef](https://www.chef.io/blog/5-ways-to-deal-with-the-install-sh-curl-pipe-bash-problem)

## 3. Package managers + release tooling

If you want a more “native” experience per OS without users manually touching binaries, the next conventions are:

- Homebrew formula for macOS (and Linuxbrew)
- `.deb` / `.rpm` packages for Linux distros
- Scoop / Chocolatey on Windows

Tools like Goreleaser automate building cross-platform binaries and generating package manager configs and GitHub Releases artifacts. [reddit](https://www.reddit.com/r/golang/comments/c22uqs/distributing_go_apps/)

Typical user flows then look like:

- macOS:

  ```bash
  brew tap you/tap
  brew install yourtool
  ```

- Debian/Ubuntu:

  ```bash
  sudo apt install yourtool
  ```

From the user perspective, this is the simplest, but it’s heavier for you to maintain.

## 4. Alternative “wrapper” conventions

There are a couple of less-standard, but sometimes handy patterns:

- Use `npm` as a distribution wrapper for your Go binary:  
  Projects like `go-npm` let you publish a Go binary behind an `npm install -g yourtool` experience, with the npm package just downloading the right binary from your releases. [github](https://github.com/sanathkr/go-npm)
- Use something like 0install, snap, or other meta-installers; these are more niche but can give a single-command “run this app” UX. [discourse.gohugo](https://discourse.gohugo.io/t/enhanced-binary-distribution-with-0install/2798)

These are useful if your target users already live in those ecosystems (frontend folks with Node, etc.).

## 5. What I’d recommend for you

Given your audience (design agencies, dev-ish but not Go-specific) and your own stack:

1. Support `go install` for developers.  
2. Provide a `curl | sh` installer that:
   - Detects OS/arch  
   - Pulls from GitHub Releases  
   - Installs to `~/.local/bin` on Linux/macOS and prints a clear PATH hint  
3. As you get adoption, add:
   - A Homebrew tap for macOS  
   - Possibly `.deb` via Goreleaser for Ubuntu users

This gives you a very simple story in docs:

```bash
# Unix users
curl -fsSL https://get.yourtool.dev/install.sh | sh

# Go developers
go install github.com/r2ware/yourtool/cmd/yourtool@latest
```

Would your users mostly be on macOS, or are you expecting a lot of Linux and Windows desktops as well? That will change whether I’d prioritize Homebrew vs. deb/rpm vs. a Windows installer in the next step.