# `sort` filter silently ignores timestamp/date-valued keys (returns unsorted order)

## Summary

The `sort` Liquid filter does not sort a collection when the sort key holds a
YAML timestamp/date value. Instead of ordering chronologically, it returns the
documents in their default (filename) order. No error or warning is emitted, so
the page builds "successfully" with the wrong order.

`sort` works correctly for integer and string keys, so the problem is specific
to date/time values.

## Environment

- gojekyll version: `develop` (`gojekyll version`)
- go: `go1.25.4 darwin/arm64`
- OS: macOS 26.5 (build 25F71), arm64

## Steps to reproduce

`_config.yml`:

```yaml
collections:
  events:
    output: true
```

Three collection docs whose **filename order (a, m, z) is the reverse of their
chronological order**:

`_events/a.md`
```yaml
---
title: A-filename
event_date: 2026-09-01 00:00:00
rank: 30
name_key: charlie
---
```

`_events/m.md`
```yaml
---
title: M-filename
event_date: 2026-05-01 00:00:00
rank: 20
name_key: bravo
---
```

`_events/z.md`
```yaml
---
title: Z-filename
event_date: 2026-01-01 00:00:00
rank: 10
name_key: alpha
---
```

`index.html`
```liquid
---
---
by_date:   {% assign s = site.events | sort: "event_date" %}{% for e in s %}{{ e.title }} {% endfor %}
by_rank:   {% assign s = site.events | sort: "rank" %}{% for e in s %}{{ e.title }} {% endfor %}
by_string: {% assign s = site.events | sort: "name_key" %}{% for e in s %}{{ e.title }} {% endfor %}
no_sort:   {% for e in site.events %}{{ e.title }} {% endfor %}
```

Run `gojekyll build` and read `_site/index.html`.

## Actual output

```
by_date:   A-filename M-filename Z-filename
by_rank:   Z-filename M-filename A-filename
by_string: Z-filename M-filename A-filename
no_sort:   A-filename M-filename Z-filename
```

`by_date` is identical to `no_sort` (unsorted, filename order). `by_rank` and
`by_string` are correctly sorted.

## Expected output

`by_date` should be chronological, matching Ruby Jekyll:

```
by_date:   Z-filename M-filename A-filename
```

(Z = 2026-01-01, M = 2026-05-01, A = 2026-09-01.)

## Notes

- The values are valid YAML timestamps (`2026-09-01 00:00:00`), parsed into
  date/time objects. The `sort` comparator appears to have no ordering for that
  type and falls back to leaving the slice untouched, rather than erroring.
- This is the standard Jekyll idiom for ordering an events/posts collection by a
  custom date field (`{% assign x = site.events | sort: "event_date" %}`), so it
  affects a common real-world template. The page builds without warning, which
  makes the wrong order easy to ship unnoticed.
- A `sort_natural` / string-coerced workaround is not equivalent, since
  lexical sort of `YYYY-MM-DD HH:MM:SS` only happens to work for zero-padded,
  same-format values.

## Possible fix direction

Ensure the `sort` filter's comparator handles `time.Time` (and date) values,
comparing with `.Before()` / `.After()`, consistent with how it already handles
ints and strings.
