---
layout: splash
title: Jigyll
description: Jigyll is a static site generator written in Go, with Markdown, Liquid templates, partial Jekyll compatibility, and live reload.
nav_exclude: true
hero:
  title: A static site generator in Go.
  tagline: Jigyll builds sites from Markdown, Liquid templates, and YAML front matter. It implements part of Jekyll; Ruby plugins are not supported.
  image:
    src: /assets/paper-dragon-beaker.webp
    alt: A blue-and-white paper dragon wearing safety goggles and experimenting with a bubbling flask.
    width: 971
    height: 861
  actions:
    - text: Installation
      link: /docs/installation/
      variant: primary
    - text: Documentation
      link: /docs/
      variant: secondary
---

{% include jigyll/assets.html %}

<div class="splash-features not-content" aria-label="Working with Jigyll">
  <div>
    <span class="splash-eyebrow">01 / Compatibility</span>
    <h2>Jekyll source conventions</h2>
    <p>Use <code>_config.yml</code>, layouts, includes, and collections. Template behavior and built-in plugin support differ from Ruby Jekyll.</p>
    <a href="{{ '/docs/differences/' | relative_url }}">Compatibility reference <span aria-hidden="true">→</span></a>
  </div>
  <div>
    <span class="splash-eyebrow">02 / Dependencies</span>
    <h2>No Ruby runtime</h2>
    <p>Run the compiled binary without Ruby or Bundler. SCSS compilation requires the Dart Sass executable.</p>
    <a href="{{ '/docs/installation/' | relative_url }}">Installation methods <span aria-hidden="true">→</span></a>
  </div>
  <div>
    <span class="splash-eyebrow">03 / Development</span>
    <h2>Build and preview</h2>
    <p><code>jigyll build</code> writes to <code>_site/</code>. <code>jigyll serve</code> previews locally, watches source files, and reloads the browser on changes.</p>
    <a href="{{ '/docs/usage/' | relative_url }}">CLI reference <span aria-hidden="true">→</span></a>
  </div>
</div>

<section class="splash-performance not-content" aria-labelledby="performance-title">
  <div class="splash-performance-intro">
    <p class="splash-eyebrow">Benchmarks</p>
    <h2 id="performance-title">Full-build timings</h2>
    <p>Two local workloads built with Ruby Jekyll and Jigyll. Output was checked for equivalence before timing.</p>
    <a href="{{ '/docs/benchmarks/#local-comparison-november-7-2026' | relative_url }}">Methodology and results <span aria-hidden="true">→</span></a>
  </div>
  <figure class="perf-chart" aria-labelledby="perf-chart-title" aria-describedby="perf-chart-note">
    <figcaption id="perf-chart-title">Full-build time <span>Lower is better</span></figcaption>
    <div class="perf-chart-panel">
      <div class="perf-chart-heading">
        <h3>2,000 posts · 20 pages</h3>
        <strong>4.70× <span>speedup</span></strong>
      </div>
      <div class="perf-chart-row">
        <span>Ruby Jekyll</span>
        <div class="perf-chart-track" aria-hidden="true"><span class="perf-chart-bar perf-chart-bar--jekyll" style="width: 100%"></span></div>
        <span class="perf-chart-time">4.308 s</span>
      </div>
      <div class="perf-chart-row">
        <span>Jigyll</span>
        <div class="perf-chart-track" aria-hidden="true"><span class="perf-chart-bar perf-chart-bar--jigyll" style="width: 21.297%"></span></div>
        <span class="perf-chart-time">0.918 s</span>
      </div>
    </div>
    <div class="perf-chart-panel">
      <div class="perf-chart-heading">
        <h3>Small Sass site · 14 output files</h3>
        <strong>35.17× <span>speedup</span></strong>
      </div>
      <div class="perf-chart-row">
        <span>Ruby Jekyll</span>
        <div class="perf-chart-track" aria-hidden="true"><span class="perf-chart-bar perf-chart-bar--jekyll" style="width: 100%"></span></div>
        <span class="perf-chart-time">359.5 ms</span>
      </div>
      <div class="perf-chart-row">
        <span>Jigyll</span>
        <div class="perf-chart-track" aria-hidden="true"><span class="perf-chart-bar perf-chart-bar--jigyll" style="width: 2.843%"></span></div>
        <span class="perf-chart-time">10.22 ms</span>
      </div>
    </div>
    <p id="perf-chart-note" class="perf-chart-note">Mean of 10 full builds after two warmups, default settings. Each pair starts at zero and uses its own linear scale; Ruby Jekyll = 100%.</p>
  </figure>
  <p class="splash-performance-caveat">Local observations, not a cross-machine guarantee. Timings include fresh CLI startup and output writes, with warm application/OS caches; these are not incremental builds. Setup and compilation are excluded. Startup and cache effects matter especially on the small Sass site. Runs were not quiet-host, reversed-order experiments. <a href="{{ '/docs/benchmarks/#measured-build-times' | relative_url }}">See variability and diagnostic results.</a></p>
</section>

{% include jigyll/site-links.html %}
