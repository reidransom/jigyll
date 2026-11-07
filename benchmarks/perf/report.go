package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Sample statistics are computed from raw times, never Hyperfine's summary or
// fastest-command ratio. A single observation has no sample standard deviation.
type sampleStats struct {
	Command string    `json:"command"`
	Times   []float64 `json:"times_seconds"`
	Count   int       `json:"count"`
	Mean    float64   `json:"mean_seconds"`
	Stddev  *float64  `json:"sample_stddev_seconds"`
	Median  float64   `json:"median_seconds"`
	Ratio   *float64  `json:"mean_jekyll_over_mean_jigyll,omitempty"`
}

type rawResult struct {
	Command   string     `json:"command"`
	Times     []*float64 `json:"times"`
	ExitCodes []*int     `json:"exit_codes"`
}

func selectedEngines(profile string) []string {
	if profile == "all" {
		return []string{"jekyll", "jigyll-default", "jigyll-single", "jigyll-no-cache", "jigyll-single-no-cache"}
	}
	return []string{"jekyll", "jigyll-" + profile}
}

func summarize(raw []byte, runs int, profile string) ([]sampleStats, error) {
	var data struct {
		Results []rawResult `json:"results"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("malformed Hyperfine JSON: %w", err)
	}
	expected := selectedEngines(profile)
	if runs < 1 || len(data.Results) != len(expected) {
		return nil, fmt.Errorf("incomplete result set")
	}
	byName := map[string]sampleStats{}
	for _, r := range data.Results {
		if !contains(expected, r.Command) {
			return nil, fmt.Errorf("unexpected result %q", r.Command)
		}
		if _, exists := byName[r.Command]; exists {
			return nil, fmt.Errorf("duplicate result %q", r.Command)
		}
		if len(r.Times) != runs || len(r.ExitCodes) != runs {
			return nil, fmt.Errorf("incomplete samples for %s", r.Command)
		}
		s := sampleStats{Command: r.Command, Count: runs, Times: make([]float64, runs)}
		// Welford's recurrence avoids subtracting nearly equal squared sums.
		var m2 float64
		for i, t := range r.Times {
			if t == nil || math.IsNaN(*t) || math.IsInf(*t, 0) || *t <= 0 || r.ExitCodes[i] == nil || *r.ExitCodes[i] != 0 {
				return nil, fmt.Errorf("invalid or failed sample for %s at %d", r.Command, i+1)
			}
			s.Times[i] = *t
			delta := *t - s.Mean
			s.Mean += delta / float64(i+1)
			m2 += delta * (*t - s.Mean)
		}
		if runs > 1 {
			deviation := math.Sqrt(m2 / float64(runs-1))
			s.Stddev = &deviation
		}
		sorted := append([]float64(nil), s.Times...)
		sort.Float64s(sorted)
		s.Median = sorted[runs/2]
		if runs%2 == 0 {
			s.Median = sorted[runs/2-1]/2 + sorted[runs/2]/2
		}
		if math.IsInf(s.Mean, 0) || math.IsNaN(s.Mean) || (s.Stddev != nil && (math.IsInf(*s.Stddev, 0) || math.IsNaN(*s.Stddev))) {
			return nil, fmt.Errorf("nonfinite statistics for %s", r.Command)
		}
		byName[r.Command] = s
	}
	out := make([]sampleStats, 0, len(expected))
	for _, name := range expected {
		s, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("missing result %s", name)
		}
		if name != "jekyll" {
			ratio := byName["jekyll"].Mean / s.Mean
			if math.IsInf(ratio, 0) || math.IsNaN(ratio) || ratio <= 0 {
				return nil, fmt.Errorf("nonfinite ratio for %s", name)
			}
			s.Ratio = &ratio
		}
		out = append(out, s)
	}
	return out, nil
}

type measuredCommand struct {
	Name        string            `json:"name"`
	Command     string            `json:"command"`
	Wrapper     string            `json:"wrapper"`
	Destination string            `json:"destination"`
	Environment map[string]string `json:"environment"`
}

type runMetadata struct {
	Schema         int               `json:"schema_version"`
	Outcome        string            `json:"outcome"`
	Started        string            `json:"timestamp_before_measurement"`
	Finished       string            `json:"timestamp_after_measurement,omitempty"`
	Run            map[string]string `json:"run"`
	Host           map[string]string `json:"host"`
	Tools          map[string]string `json:"tools"`
	Checkout       map[string]string `json:"jigyll_checkout"`
	Environment    map[string]string `json:"environment"`
	SourceHashes   map[string]string `json:"source_sha256"`
	InputHashes    map[string]string `json:"input_sha256"`
	ArtifactHashes map[string]string `json:"artifact_sha256,omitempty"`
	Generator      json.RawMessage   `json:"generator,omitempty"`
	Commands       []measuredCommand `json:"commands"`
}

func digest(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func hashFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return digest(b), nil
}

// Never enumerate the ambient environment. These are the effective, relevant
// settings used by the harness; arbitrary BUNDLE_*, tokens and PATH are excluded.
func relevantEnvironment(getenv func(string) string) map[string]string {
	out := map[string]string{}
	for _, key := range []string{"LC_ALL", "LANG", "TZ", "JEKYLL_ENV", "BUNDLE_GEMFILE", "BUNDLE_PATH", "BUNDLE_FROZEN", "BUNDLE_IGNORE_CONFIG", "BUNDLE_VERSION", "GEM_HOME", "GEM_PATH"} {
		out[key] = getenv(key)
	}
	for _, key := range []string{"GOFLAGS", "GOOS", "GOARCH", "GOEXPERIMENT", "GOMAXPROCS", "GOGC", "GOMEMLIMIT", "GODEBUG", "GOTRACEBACK", "JIGYLL_DISABLE_CACHE", "GOJEKYLL_DISABLE_CACHE", "JEKYLL_CONFIG", "JEKYLL_URL", "RUBYOPT", "RUBYLIB"} {
		out[key] = "unset (runtime default)"
	}
	return out
}

func commandOutput(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("provenance command %s failed: %w", name, err)
	}
	return strings.TrimSpace(string(b)), nil
}

func readRun(root string) (map[string]string, error) {
	b, err := os.ReadFile(filepath.Join(root, "run.tsv"))
	if err != nil {
		return nil, err
	}
	allowed := []string{"schema_version", "source_original", "source", "site_argument", "runs", "warmup", "profile", "version", "build_date", "bundle_gemfile", "bundle_path", "jekyll_disk_cache"}
	out := map[string]string{}
	s := bufio.NewScanner(bytes.NewReader(b))
	for s.Scan() {
		parts := strings.Split(s.Text(), "\t")
		if len(parts) != 2 || !contains(allowed, parts[0]) || parts[1] == "" || out[parts[0]] != "" {
			return nil, fmt.Errorf("invalid run.tsv field")
		}
		out[parts[0]] = parts[1]
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	if len(out) != len(allowed) || out["schema_version"] != "1" {
		return nil, fmt.Errorf("incomplete run.tsv")
	}
	if err := validate(out["runs"], out["warmup"], out["profile"]); err != nil {
		return nil, err
	}
	if out["source"] != filepath.Join(root, "source") {
		return nil, fmt.Errorf("source is not the owned copy")
	}
	return out, nil
}

func readCommands(root, profile string) ([]measuredCommand, error) {
	b, err := os.ReadFile(filepath.Join(root, "commands.tsv"))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	expected := selectedEngines(profile)
	if len(lines) != len(expected)+1 || lines[0] != "name\tcommand\tdestination\ttmpdir\txdg_cache_home\tGOMAXPROCS\tJIGYLL_DISABLE_CACHE" {
		return nil, fmt.Errorf("invalid command handoff")
	}
	out := []measuredCommand{}
	for i, name := range expected {
		p := strings.Split(lines[i+1], "\t")
		if len(p) != 7 || p[0] != name || p[2] != filepath.Join(root, "output", name) || p[3] != filepath.Join(root, "cache", name, "tmp") || p[4] != filepath.Join(root, "cache", name, "xdg") {
			return nil, fmt.Errorf("invalid command locations for %s", name)
		}
		maxprocs, disabled := "default", "0"
		if name == "jigyll-single" || name == "jigyll-single-no-cache" {
			maxprocs = "1"
		}
		if name == "jigyll-no-cache" || name == "jigyll-single-no-cache" {
			disabled = "1"
		}
		if p[5] != maxprocs || p[6] != disabled {
			return nil, fmt.Errorf("invalid profile settings for %s", name)
		}
		wrapper, err := os.ReadFile(filepath.Join(root, "commands", name))
		if err != nil {
			return nil, err
		}
		env := map[string]string{"GOMAXPROCS": maxprocs, "JIGYLL_DISABLE_CACHE": disabled, "GOGC": "unset (normal GC)", "GOMEMLIMIT": "unset (runtime default)", "TMPDIR": p[3], "XDG_CACHE_HOME": p[4]}
		for key, child := range map[string]string{"HOME": "home", "XDG_CONFIG_HOME": "config", "BUNDLE_USER_HOME": "bundle-user"} {
			env[key] = filepath.Join(root, "cache", name, child)
		}
		out = append(out, measuredCommand{name, p[1], string(wrapper), p[2], env})
	}
	return out, nil
}

func loadSnapshot() string {
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		return strings.TrimSpace(string(b))
	}
	if s, err := commandOutput("", "uptime"); err == nil {
		return s
	}
	return "unavailable on this host"
}

func cpuModel() string {
	if b, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			p := strings.SplitN(line, ":", 2)
			if len(p) == 2 && (strings.TrimSpace(p[0]) == "model name" || strings.TrimSpace(p[0]) == "Hardware") {
				return strings.TrimSpace(p[1])
			}
		}
	}
	if s, err := commandOutput("", "sysctl", "-n", "machdep.cpu.brand_string"); err == nil {
		return s
	}
	return "unavailable on this host"
}

func saveJSON(path string, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}

// Capture before any engine is run, so input hashes cannot include runtime
// caches. Only hashes of local diffs are persisted, never their potentially
// confidential contents or untracked filenames.
func captureMetadata(root, repo string) error {
	if err := owned(root); err != nil {
		return err
	}
	r, err := readRun(root)
	if err != nil {
		return err
	}
	commands, err := readCommands(root, r["profile"])
	if err != nil {
		return err
	}
	m := runMetadata{Schema: 1, Outcome: "not completed; no valid comparison", Started: time.Now().UTC().Format(time.RFC3339Nano), Run: r, Commands: commands, Environment: relevantEnvironment(os.Getenv), Tools: map[string]string{}, Checkout: map[string]string{}, SourceHashes: map[string]string{}, InputHashes: map[string]string{}}
	m.Host = map[string]string{"os": runtime.GOOS, "architecture": runtime.GOARCH, "cpu_model": cpuModel(), "logical_processors": strconv.Itoa(runtime.NumCPU()), "load_before": loadSnapshot(), "destination_root": filepath.Join(root, "output")}
	for key, args := range map[string][]string{"kernel": {"uname", "-a"}, "destination_filesystem": {"df", "-P", filepath.Join(root, "output")}} {
		value, err := commandOutput(repo, args[0], args[1:]...)
		if err != nil {
			return err
		}
		m.Host[key] = value
	}
	for key, args := range map[string][]string{"go": {"go", "version"}, "ruby": {"ruby", "--version"}, "bundler": {"bundle", "_2.6.9_", "--version"}, "jekyll": {"bundle", "_2.6.9_", "exec", "jekyll", "--version"}, "sass_jigyll": {"sass", "--version"}, "sass_jekyll_gem": {"bundle", "_2.6.9_", "exec", "ruby", "-e", "puts Gem.loaded_specs.fetch('sass-embedded').version"}, "hyperfine": {"hyperfine", "--version"}, "jigyll": {"jigyll", "--version"}} {
		if key == "jigyll" {
			args[0] = filepath.Join(root, "jigyll")
		}
		value, err := commandOutput(repo, args[0], args[1:]...)
		if err != nil {
			return err
		}
		m.Tools[key] = value
		resolved, err := exec.LookPath(args[0])
		if err != nil {
			return err
		}
		m.Tools[key+"_executable"] = resolved
	}
	for key, args := range map[string][]string{"commit": {"rev-parse", "HEAD"}, "status": {"status", "--porcelain=v1", "-z", "--untracked-files=all"}, "tracked_diff": {"diff", "HEAD", "--binary"}, "untracked": {"ls-files", "--others", "--exclude-standard", "-z"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		b, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("git provenance %s: %w", key, err)
		}
		switch key {
		case "commit":
			m.Checkout[key] = strings.TrimSpace(string(b))
		case "status":
			m.Checkout["dirty"] = strconv.FormatBool(len(b) > 0)
			m.Checkout["status_sha256"] = digest(b)
		case "tracked_diff":
			m.Checkout["tracked_diff_sha256"] = digest(b)
		case "untracked":
			h := sha256.New()
			count := 0
			for _, p := range strings.Split(string(b), "\x00") {
				if p == "" {
					continue
				}
				sum, err := hashFile(filepath.Join(repo, p))
				if err != nil {
					return err
				}
				fmt.Fprintf(h, "%s\x00%s\n", p, sum)
				count++
			}
			m.Checkout["untracked_content_sha256"] = hex.EncodeToString(h.Sum(nil))
			m.Checkout["untracked_file_count"] = strconv.Itoa(count)
		}
	}
	paths, err := files(r["source"])
	if err != nil {
		return err
	}
	for _, p := range paths {
		sum, err := hashFile(filepath.Join(r["source"], p))
		if err != nil {
			return err
		}
		m.SourceHashes[p] = sum
	}
	inputs := map[string]string{"binary": filepath.Join(root, "jigyll"), "shared_config": filepath.Join(root, "common.yml"), "gemfile": r["bundle_gemfile"], "lockfile": r["bundle_gemfile"] + ".lock", "generator_source": filepath.Join(repo, "benchmarks/generate/main.go"), "go_mod": filepath.Join(repo, "go.mod"), "go_sum": filepath.Join(repo, "go.sum")}
	for _, c := range commands {
		inputs["config/"+c.Name] = filepath.Join(root, "config", c.Name+".yml")
		inputs["commands/"+c.Name] = filepath.Join(root, "commands", c.Name)
	}
	for key, p := range inputs {
		sum, err := hashFile(p)
		if err != nil {
			return err
		}
		m.InputHashes[key] = sum
	}
	if b, err := os.ReadFile(filepath.Join(r["source"], "_benchmark/manifest.json")); err == nil {
		var manifest struct {
			Revision   string          `json:"generator_revision"`
			Upstream   string          `json:"upstream_revision"`
			Parameters json.RawMessage `json:"parameters"`
		}
		if err := json.Unmarshal(b, &manifest); err != nil {
			return err
		}
		m.Generator, err = json.Marshal(manifest)
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return saveJSON(filepath.Join(root, "metadata.json"), m)
}

func cacheLabel(warmup int) string {
	if warmup == 0 {
		return "No warmup: application-cold start after preflight reset; later samples reuse application/OS caches. OS caches were not dropped."
	}
	return fmt.Sprintf("%d warmups: repeated-process application/OS cache effects, not incremental rendering. OS caches were not dropped.", warmup)
}

func writeReport(root string) error {
	if err := owned(root); err != nil {
		return err
	}
	if b, err := os.ReadFile(filepath.Join(root, "status")); err == nil && strings.TrimSpace(string(b)) != "success" {
		return fmt.Errorf("failed/incomplete run status; no success report")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	r, err := readRun(root)
	if err != nil {
		return err
	}
	for _, path := range []string{"preflight-reset", "verification.log", "hyperfine.md", "hyperfine-command"} {
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return err
		}
		if len(bytes.TrimSpace(b)) == 0 {
			return fmt.Errorf("missing/incomplete %s", path)
		}
	}
	b, err := os.ReadFile(filepath.Join(root, "metadata.json"))
	if err != nil {
		return err
	}
	var m runMetadata
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	if m.Schema != 1 || !reflect.DeepEqual(m.Run, r) || (m.Outcome != "not completed; no valid comparison" && m.Outcome != "success") {
		return fmt.Errorf("metadata does not match measured run")
	}
	if _, err := time.Parse(time.RFC3339Nano, m.Started); err != nil {
		return fmt.Errorf("missing/invalid measurement timestamp")
	}
	commands, err := readCommands(root, r["profile"])
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(commands, m.Commands) {
		return fmt.Errorf("metadata command mismatch")
	}
	for _, group := range []struct {
		values   map[string]string
		required []string
	}{
		{m.Host, []string{"os", "architecture", "cpu_model", "logical_processors", "load_before", "kernel", "destination_filesystem", "destination_root"}},
		{m.Tools, []string{"go", "ruby", "bundler", "jekyll", "sass_jigyll", "sass_jekyll_gem", "hyperfine", "jigyll"}},
		{m.Checkout, []string{"commit", "dirty", "status_sha256", "tracked_diff_sha256", "untracked_content_sha256", "untracked_file_count"}},
		{m.InputHashes, []string{"binary", "shared_config", "gemfile", "lockfile", "generator_source", "go_mod", "go_sum"}},
	} {
		for _, key := range group.required {
			if group.values[key] == "" {
				return fmt.Errorf("incomplete provenance: %s", key)
			}
		}
	}
	if len(m.SourceHashes) == 0 {
		return fmt.Errorf("missing source identity")
	}
	for p, sum := range m.SourceHashes {
		if !relativePath(filepath.FromSlash(p)) {
			return fmt.Errorf("invalid source identity path")
		}
		actual, err := hashFile(filepath.Join(r["source"], p))
		if err != nil {
			return err
		}
		if actual != sum {
			return fmt.Errorf("source changed during measurement: %s", p)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "hyperfine.json"))
	if err != nil {
		return err
	}
	runs, _ := strconv.Atoi(r["runs"])
	warmup, _ := strconv.Atoi(r["warmup"])
	stats, err := summarize(raw, runs, r["profile"])
	if err != nil {
		return err
	}
	m.Finished = time.Now().UTC().Format(time.RFC3339Nano)
	m.Host["load_after"] = loadSnapshot()
	m.Outcome = "success"
	m.ArtifactHashes = map[string]string{}
	for _, p := range []string{"hyperfine.json", "hyperfine.md", "hyperfine-command", "verification.log"} {
		sum, err := hashFile(filepath.Join(root, p))
		if err != nil {
			return err
		}
		m.ArtifactHashes[p] = sum
	}
	var report strings.Builder
	fmt.Fprintf(&report, "# Native full-build comparison\n\nWorkload: `%s` (original `%s`, measured private copy `%s`).\n\n", r["site_argument"], r["source_original"], r["source"])
	if len(m.Generator) > 0 {
		fmt.Fprintf(&report, "Generated workload identity/parameters: `%s`. This fixture has no Sass; these diagnostics cannot demonstrate Sass-cache benefits.\n\n", m.Generator)
	}
	fmt.Fprintf(&report, "Boundary: fresh native CLI processes, precompiled Jigyll versus Bundler + Jekyll startup and full build, including native destination cleanup/writes. Compilation, dependency setup, source generation, preflight and reporting are excluded. Shared policy: future=true, incremental=false, watch=false, UTC, production. Prior outputs and private per-profile caches are retained between samples; preflight state is reset first.\n\nMeasured samples per command: %d; requested warmups: %d. %s\n\n", runs, warmup, cacheLabel(warmup))
	report.WriteString("Times in seconds; sample standard deviation uses n−1 (undefined for n=1). Raw sample order is preserved in statistics.json and hyperfine.json.\n\n| Command | Mean ± sample stddev (s) | Median (s) | mean(Jekyll) / mean(Jigyll) |\n| --- | --- | --- | --- |\n")
	for _, s := range stats {
		deviation, ratio := "undefined (n=1)", "baseline"
		if s.Stddev != nil {
			deviation = strconv.FormatFloat(*s.Stddev, 'g', 17, 64)
		}
		if s.Ratio != nil {
			ratio = strconv.FormatFloat(*s.Ratio, 'g', 17, 64)
		}
		fmt.Fprintf(&report, "| %s | %.17g ± %s | %.17g | %s |\n", s.Command, s.Mean, deviation, s.Median, ratio)
	}
	report.WriteString("\nRatio below one means Jigyll is slower. Ratios are descriptive sample means, not proof of improvement or statistical significance. Small/noisy or overlapping samples must not be presented as demonstrated speedups. Non-default Jigyll profiles are diagnostics against the same single Ruby baseline, not matching Ruby cache/thread modes. JIGYLL_DISABLE_CACHE=1 disables reuse, not all cache I/O. No historical upstream numbers are measured here. Publication requires quiet-host repeats with reversed command order and retained artifacts.\n\n## Exact commands and settings\n\nThese wrappers include exact argv, working directory, environment normalization, profile settings and log redirection. To rerun standalone, source scripts/perf-env and select the recorded locked bundle first.\n")
	for _, c := range m.Commands {
		fmt.Fprintf(&report, "\n### %s\n\nHyperfine command: `%s`\n\n```bash\n%s```\n", c.Name, c.Command, c.Wrapper)
	}
	hyperfineCommand, err := os.ReadFile(filepath.Join(root, "hyperfine-command"))
	if err != nil {
		return err
	}
	fmt.Fprintf(&report, "\nHyperfine invocation (outside compilation/setup):\n\n```bash\n%s```\n", hyperfineCommand)
	fmt.Fprintf(&report, "\n## Provenance and evidence\n\nStarted: %s; finished: %s.\n\nCommit: `%s`; local changes: %s; measured binary SHA-256: `%s`.\n\nTool versions: Go `%s`; Ruby `%s`; Bundler `%s`; Jekyll `%s`; Jigyll Sass `%s`; Ruby Sass gem `%s`; Hyperfine `%s`; Jigyll `%s`.\n\nSee metadata.json for host/kernel/CPU/load/filesystem, allowlisted effective environment, generator provenance, full copied-source/config/lock hashes, binary and local-change digests, and raw artifact hashes. verification.log records output equivalence. hyperfine.json, hyperfine.md, hyperfine.log and logs/ preserve raw evidence. Artifact directory: `%s`.\n", m.Started, m.Finished, m.Checkout["commit"], m.Checkout["dirty"], m.InputHashes["binary"], m.Tools["go"], m.Tools["ruby"], m.Tools["bundler"], m.Tools["jekyll"], m.Tools["sass_jigyll"], m.Tools["sass_jekyll_gem"], m.Tools["hyperfine"], m.Tools["jigyll"], root)
	if err := saveJSON(filepath.Join(root, "statistics.json"), stats); err != nil {
		return err
	}
	// Do not mark provenance successful until the readable report was written.
	// The harness quarantines any partial result artifacts if publication fails.
	if err := os.WriteFile(filepath.Join(root, "report.md"), []byte(report.String()), 0600); err != nil {
		return err
	}
	return saveJSON(filepath.Join(root, "metadata.json"), m)
}
