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
make setup
go get -t ./...
```

[Install golangci-lint](https://golangci-lint.run/usage/install/#local-installation).
On macOS: `brew install golangci-lint`

Install the Dart Sass executable (required for tests):
- On macOS: `brew install sass/sass/sass`
- On Linux: `wget -qO- https://github.com/sass/dart-sass/releases/download/1.66.1/dart-sass-1.66.1-linux-x64.tar.gz | tar -xz && sudo mv dart-sass/* /usr/bin/ && rmdir dart-sass`

### Test and Lint

```bash
make test
make lint
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

Benchmarks are listed in the file ./docs/benchmarks.md.

As of 2022-02, I use [hyperfine](https://github.com/sharkdp/hyperfine) for
benchmarking. (I don't remember what I used for previous benchmarks.)

The "single-threaded" and "cached disabled" benchmarks use these settings:

* Cache disabled: Disable the cache by setting the environment variable
  `JIGYLL_DISABLE_CACHE=1`.
* Single-threaded: Disable threading by setting `GOMAXPROCS=1`.

For example:

```sh
GOMAXPROCS=1 JIGYLL_DISABLE_CACHE=1 hyperfine --warmup 2 "jigyll build -s ${SITE_SRC}"
GOMAXPROCS=1 hyperfine --warmup 2 "jigyll build -s ${SITE_SRC}"
JIGYLL_DISABLE_CACHE=1 hyperfine --warmup 2 "jigyll build -s ${SITE_SRC}"
hyperfine --warmup 2 "jigyll build -s ${SITE_SRC}"
```

If you run into an error after a few runs, add the `--show-ouput` option to
`hyperfine`.

### Coverage

```bash
./scripts/coverage && go tool cover -html=coverage.out
```

### Profiling

```bash
jigyll -s path/to/site benchmark
go tool pprof --web jigyll jigyll.prof
```
