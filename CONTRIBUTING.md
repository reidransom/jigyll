# Contributing

Here's some ways to help:

* Try using jigyll on your site. Use this as fodder for test cases.
* Choose an item to work on from the [issues list](https://github.com/reidransom/jigyll/issues).
* Search the sources for FIXME and TODO comments.
* Improve the [code coverage](https://coveralls.io/github/reidransom/jigyll?branch=main).

If you choose to contribute code, please review the [pull request
template](https://github.com/reidransom/jigyll/blob/main/.github/PULL_REQUEST_TEMPLATE.md)
before you get too far along.

## Developer Cookbook

### Set up your machine

Fork and clone the repo.

[Install go](https://golang.org/doc/install#install). On macOS running Homebrew,
`brew install go` is easier than the linked instructions.

Install package dependencies and development tools:

```bash
just setup
```

[Install golangci-lint](https://golangci-lint.run/usage/install/#local-installation).
On macOS: `brew install golangci-lint`

Install the Dart Sass executable (required for tests and benchmarks) at the
project's `mise.toml` pin, **1.102.0**:

```bash
mise install github:sass/dart-sass@1.102.0
```

### Test and Lint

```bash
just test
just lint
```

### Debugging tools

```bash
jigyll -s path/to/site render index.md              # render a file to stdout
jigyll -s path/to/site render /                     # render a URL to stdout
jigyll -s path/to/site variables /                  # print a file or URL's variables
jigyll -s path/to/site variables site               # print the site variables
jigyll -s path/to/site variables site.twitter.name  # print a specific site variable
```

`./scripts/jigyll` is an alternative to the `jigyll` executable, that uses
`go run` each time it's invoked.

### Benchmarks

Use [`just perf`](docs/_docs/benchmarks.md) to compare this checkout with native
Ruby Jekyll. The linked page separates historical upstream Gojekyll tables from
current measurements. Run the following commands from the repository root in Bash.

#### Install and prepare outside timing

Prerequisites on PATH: `just`, Git, Go (matching `go.mod`), `mise`, Cargo/Rust,
`cc`, `pkg-config`, and Dart Sass **1.102.0** (installation above). Initial setup
needs access to mise's precompiled Ruby distribution, RubyGems and Cargo's registry;
native gems need the C toolchain. No root access or system Ruby replacement is
required. Setup fails explicitly if a precompiled Ruby is unavailable.

```bash
just perf-setup
```

The verified pins are Ruby **3.4.7**, Bundler **2.6.9**, Jekyll **4.4.1**,
Hyperfine **1.19.0**, and Ruby's `sass-embedded` **1.102.0**.
`benchmarks/Gemfile.lock` pins the generated site's complete gem graph.
Hyperfine is installed from its released crate with `cargo install --locked`.
All installations are private beneath `.scratch/performance-comparison/tools/`:

| Component | Relative location beneath `tools/` |
| --- | --- |
| Ruby | `mise/installs/ruby/3.4.7/bin/` |
| Bundler gems | `gems/ruby-3.4.7/` |
| Hyperfine | `hyperfine/1.19.0/bin/` |
| Generated-site bundle | `bundles/generated/` |
| Site-specific bundles | `bundles/sites/<Gemfile-and-lock-content-key>/` |

Setup also creates `.scratch/performance-comparison/generated/source/`: a
deterministic, network-free fixture with 2,000 posts, 20 standalone pages plus
the index, 10 includes and four layouts. Existing reports are preserved; an
incompatible existing fixture must be moved aside explicitly, not overwritten.
Sourcing `scripts/perf-env` selects these tools without installing anything:

```bash
(
  source scripts/perf-env
  perf_use_bundle "$PWD/benchmarks/Gemfile"
  ruby --version
  bundle _2.6.9_ --version
  bundle _2.6.9_ exec jekyll --version
  bundle _2.6.9_ exec ruby -e 'puts Gem.loaded_specs.fetch("sass-embedded").version'
  hyperfine --version
  sass --version
)
```

#### Run full builds

Argument order is `just perf [site] [runs] [warmup] [profile]`. Defaults are
`generated`, `10`, `2`, `default`. Runs must be positive; warmups may be zero.
Quote site paths containing spaces.

```bash
just perf
just perf generated 10 2 all
```

| Profile | Jigyll settings |
| --- | --- |
| `default` | Normal concurrency and persistent-cache reuse |
| `single` | `GOMAXPROCS=1` |
| `no-cache` | `JIGYLL_DISABLE_CACHE=1` |
| `single-no-cache` | Both overrides |
| `all` | All four Jigyll configurations, one Ruby baseline |

These are Jigyll diagnostics, not equivalent Jekyll thread/cache modes.
Disabling cache reuse does not disable cache I/O. The generated fixture has no
Sass, so it cannot establish Sass-cache benefits.

For an existing site, preserve its Gemfile **and** lockfile and prepare its own
bundle first. Resolve any missing/incompatible lock outside timing; setup uses
frozen dependencies and never rewrites the site's lock or Bundler configuration.
Replace the example path with your absolute site path:

```bash
site="/absolute/path/to/site with spaces"
scripts/perf-setup "$site"
just perf "$site" 10 2 all
```

A site without either dependency file uses the generated baseline bundle.
A site with only one of Gemfile/lockfile is rejected. `just perf-setup` alone
does not prepare an arbitrary site's bundle. `perf` checks dependencies but
never installs them. Resolve plugins, remote themes and build-time network
dependencies before comparing; do not remove work for only one engine.
See the [verified Sass preparation](docs/_docs/benchmarks.md#verified-real-site-preparation)
for a concrete compatible pinned site and the limits of historical fixtures.

#### Measurement boundary and fairness

The harness compiles this checkout once, without `go mod tidy`, race
instrumentation or inherited Go diagnostic overrides. It times fresh native
processes: that precompiled Jigyll binary versus
`bundle _2.6.9_ exec jekyll build`, including Ruby/Bundler startup. Compilation,
installation, generation, preflight and reporting are outside timing. Both
engines perform full builds with real destination cleanup/writes; this is not
`go run`, `jigyll benchmark`, Docker, a dry run or an incremental/watch loop.

Both use the same private source copy and configuration override:
`future: true`, `incremental: false`, `watch: false`, UTC, `LC_ALL=C`,
`LANG=C`, and `JEKYLL_ENV=production`. Including future posts prevents larger
generated workloads from silently omitting work. Real-site copying excludes old
destinations, caches, VCS internals and dependency directories, without modifying
the original source.

Preflight builds both engines and checks intended URL coverage and representative
rendered content, not just file counts. Generated checks cover every post/page,
titles, bodies, nested layouts, includes and navigation. Unsupported differences
or failed builds invalidate the comparison and return nonzero.

Preflight output/application caches are reset before warmups. Each invocation
and each engine/profile has private destinations, TMPDIR and cache roots.
Subsequent warmups/samples retain prior output and application caches, including
ordinary full-rebuild cleanup in the timed command. No shared developer cache
or host OS cache is cleared. Two warmups mean repeated-process application/OS
cache effects, not incremental rendering; zero warmups do not mean warm-cache
or cold-OS-cache measurements. Normal GC/concurrency apply unless the selected
profile overrides them; inherited Go GC/concurrency/cache diagnostics are unset.

#### Read the artifacts

Each invocation prints its unique `.scratch/performance-comparison/runs/run-*/`
directory. Keep the entire directory:

* `report.md`: boundary, settings, exact wrappers, mean ± sample standard
  deviation, median and `mean(Jekyll) / mean(Jigyll)`.
* `hyperfine.json` and `hyperfine.md`: raw measured samples and Hyperfine's table.
  `statistics.json` retains independently derived summary statistics and samples;
  sample standard deviation uses n−1 and is undefined for one sample.
* `metadata.json`: timestamps, host/kernel/CPU/load/filesystem, tool versions,
  commit/local-change provenance, binary/source/config/lock hashes, generator
  parameters, effective environment and artifact hashes.
* `hyperfine-command`, `commands/`, `verification.log`, `logs/`, `status`:
  exact invocation/wrappers, equivalence checks and native build diagnostics.
  `source/`, `output/` and `cache/` retain the measured private inputs/state.

A ratio below one means Jigyll is slower. Dispersion, host load and order effects
matter; overlapping/noisy samples are not proof of improvement. Failed comparisons
retain diagnostics, with partial exports marked `failed-*`, not a valid report or
ratio. Hyperfine's correctly spelled diagnostic option is `--show-output`;
the harness already retains native logs.

Reports remain local and ignored by Git. Before publishing timings, repeat on a
quiet host in both command orders and retain both sets of artifacts. The recipe
does not provide a reverse-order argument: use the recorded wrappers and explicit
Hyperfine invocation to reverse order while preserving the recorded environment,
locked bundle and private-state/reset policy. Do not select only favorable runs.

### Coverage

```bash
./scripts/coverage && go tool cover -html=coverage.out
```

### Profiling

```bash
jigyll -s path/to/site benchmark
go tool pprof --web jigyll jigyll.prof
```
