---
layout: splash
title: Jigyll
description: Build your Jekyll-style site with a native Go binary. Keep Liquid templates, preview changes live, and compare measured full-build times.
nav_exclude: true
hero:
  title: Your Jekyll site. Built with Go.
  tagline: Keep your Markdown and Liquid templates. Jigyll is a fast, partially-compatible Jekyll clone with native builds, directory watch, and live reload.
  image:
    src: /assets/paper-dragon-beaker.webp
    alt: A blue-and-white paper dragon wearing safety goggles and experimenting with a bubbling flask.
    width: 971
    height: 861
  actions:
    - text: Install Jigyll
      link: /docs/installation/
      variant: primary
    - text: Read the docs
      link: /docs/
      variant: secondary
---

{% include jigyll/assets.html %}

<div class="splash-features not-content" aria-label="Working with Jigyll">
  <div>
    <span class="splash-eyebrow">01 / Familiar source</span>
    <h2>Liquid. Markdown. Front matter.</h2>
    <p>Bring the files you already know. Check the compatibility guide before moving an existing site.</p>
    <a href="{{ '/docs/differences/' | relative_url }}">Explore compatibility <span aria-hidden="true">→</span></a>
  </div>
  <div>
    <span class="splash-eyebrow">02 / Native builds</span>
    <h2>A binary, not a Ruby toolchain.</h2>
    <p>Run Jigyll without Ruby or Bundler. Add Dart Sass when your site needs SCSS.</p>
    <a href="{{ '/docs/installation/' | relative_url }}">Choose your install <span aria-hidden="true">→</span></a>
  </div>
  <div>
    <span class="splash-eyebrow">03 / A shorter edit loop</span>
    <h2>Save. Preview. Repeat.</h2>
    <p>Use <code>jigyll serve</code> to watch your directory and reload the browser as your site changes.</p>
    <a href="{{ '/docs/usage/' | relative_url }}">Meet the CLI <span aria-hidden="true">→</span></a>
  </div>
</div>

<section class="splash-performance not-content" aria-labelledby="performance-title">
  <div class="splash-performance-intro">
    <p class="splash-eyebrow">Measured, not promised</p>
    <h2 id="performance-title">Full builds.<br>Less waiting.</h2>
    <p>The same content, built by both engines. Two local workloads, with output verified before timing.</p>
    <a href="{{ '/docs/benchmarks/#local-comparison-november-7-2026' | relative_url }}">Read the benchmark notes <span aria-hidden="true">→</span></a>
  </div>
  <figure class="perf-chart" aria-labelledby="perf-chart-title" aria-describedby="perf-chart-note">
    <figcaption id="perf-chart-title">Full-build time <span>Lower is better</span></figcaption>
    <div class="perf-chart-panel">
      <div class="perf-chart-heading">
        <h3>2,000 posts · 20 pages</h3>
        <strong>4.70× <span>faster</span></strong>
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
        <strong>35.17× <span>faster</span></strong>
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
