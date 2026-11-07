package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func sampleJSON(a, b string) []byte {
	return []byte(fmt.Sprintf(`{"results":[{"command":"jigyll-default","times":%s,"exit_codes":[0,0,0,0]},{"command":"jekyll","times":%s,"exit_codes":[0,0,0,0]}]}`, b, a))
}

func TestStatisticsUseExactSamplesAndExplicitRatio(t *testing.T) {
	// Deliberately reverse raw command order; Jigyll is slower (ratio < 1).
	stats, err := summarize(sampleJSON(`[1,2,3,4]`, `[2,4,6,8]`), 4, "default")
	require.NoError(t, err)
	require.Equal(t, "jekyll", stats[0].Command)
	require.Equal(t, []float64{1, 2, 3, 4}, stats[0].Times)
	require.Equal(t, 4, stats[0].Count)
	require.Equal(t, 2.5, stats[0].Mean)
	require.InDelta(t, math.Sqrt(5.0/3), *stats[0].Stddev, 1e-14)
	require.Equal(t, 2.5, stats[0].Median)
	require.Nil(t, stats[0].Ratio)
	require.Equal(t, 5.0, stats[1].Mean)
	require.InDelta(t, math.Sqrt(20.0/3), *stats[1].Stddev, 1e-14)
	require.Equal(t, 5.0, stats[1].Median)
	require.Equal(t, 0.5, *stats[1].Ratio)

	odd := []byte(`{"results":[{"command":"jekyll","times":[9,1,2],"exit_codes":[0,0,0]},{"command":"jigyll-default","times":[1,1,1],"exit_codes":[0,0,0]}]}`)
	stats, err = summarize(odd, 3, "default")
	require.NoError(t, err)
	require.Equal(t, []float64{9, 1, 2}, stats[0].Times)
	require.Equal(t, 2.0, stats[0].Median)
	require.Equal(t, 0.0, *stats[1].Stddev)
	require.Equal(t, 4.0, *stats[1].Ratio)
}

func TestSingleSampleDispersionIsUndefined(t *testing.T) {
	raw := []byte(`{"results":[{"command":"jekyll","times":[2],"exit_codes":[0]},{"command":"jigyll-default","times":[4],"exit_codes":[0]}]}`)
	stats, err := summarize(raw, 1, "default")
	require.NoError(t, err)
	require.Nil(t, stats[0].Stddev)
	require.Nil(t, stats[1].Stddev)
	require.Equal(t, 0.5, *stats[1].Ratio)
}

func TestRejectMalformedIncompleteAndFailedSamples(t *testing.T) {
	valid := string(sampleJSON(`[1,2,3,4]`, `[2,4,6,8]`))
	for name, raw := range map[string]string{
		"invalid JSON":        "{",
		"missing results":     "{}",
		"null results":        `{"results":null}`,
		"missing baseline":    strings.Replace(valid, `"jekyll"`, `"other"`, 1),
		"duplicate label":     strings.Replace(valid, `"jekyll"`, `"jigyll-default"`, 1),
		"missing time":        strings.Replace(valid, `[1,2,3,4]`, `[1,2,3]`, 1),
		"extra time":          strings.Replace(valid, `[1,2,3,4]`, `[1,2,3,4,5]`, 1),
		"null time":           strings.Replace(valid, `[1,2,3,4]`, `[1,null,3,4]`, 1),
		"string time":         strings.Replace(valid, `[1,2,3,4]`, `[1,"2",3,4]`, 1),
		"zero time":           strings.Replace(valid, `[1,2,3,4]`, `[1,0,3,4]`, 1),
		"negative time":       strings.Replace(valid, `[1,2,3,4]`, `[1,-2,3,4]`, 1),
		"infinite time":       strings.Replace(valid, `[1,2,3,4]`, `[1,1e999,3,4]`, 1),
		"NaN time":            strings.Replace(valid, `[1,2,3,4]`, `[1,NaN,3,4]`, 1),
		"overflow dispersion": strings.Replace(valid, `[1,2,3,4]`, `[1,1e308,3,4]`, 1),
		"failed sample":       strings.Replace(valid, `[0,0,0,0]`, `[0,1,0,0]`, 1),
		"missing exit":        strings.Replace(valid, `[0,0,0,0]`, `[0,0,0]`, 1),
		"null exit":           strings.Replace(valid, `[0,0,0,0]`, `[0,null,0,0]`, 1),
		"no exit codes":       strings.Replace(valid, `,"exit_codes":[0,0,0,0]`, "", 1),
	} {
		t.Run(name, func(t *testing.T) { _, err := summarize([]byte(raw), 4, "default"); require.Error(t, err) })
	}
	_, err := summarize([]byte(valid), 4, "all")
	require.Error(t, err)
	_, err = summarize([]byte(valid), 0, "default")
	require.Error(t, err)
}

func TestAllProfilesShareOneBaseline(t *testing.T) {
	var results []map[string]interface{}
	for _, name := range selectedEngines("all") {
		results = append(results, map[string]interface{}{"command": name, "times": []float64{2, 4}, "exit_codes": []int{0, 0}})
	}
	b, err := json.Marshal(map[string]interface{}{"results": results})
	require.NoError(t, err)
	stats, err := summarize(b, 2, "all")
	require.NoError(t, err)
	require.Len(t, stats, 5)
	for _, s := range stats[1:] {
		require.Equal(t, 1.0, *s.Ratio)
	}
}

func TestRelevantMetadataDoesNotEnumerateConfidentialEnvironment(t *testing.T) {
	for _, key := range []string{"GITHUB_TOKEN", "AWS_SECRET_ACCESS_KEY", "BUNDLE_GEMS__EXAMPLE__COM", "UNRELATED_SECRET", "RUBYOPT", "GOFLAGS"} {
		t.Setenv(key, "confidential-sentinel")
	}
	t.Setenv("TZ", "Etc/UTC")
	t.Setenv("JEKYLL_ENV", "production")
	m := relevantEnvironment(os.Getenv)
	require.Equal(t, "Etc/UTC", m["TZ"])
	require.Equal(t, "production", m["JEKYLL_ENV"])
	require.Equal(t, "unset (runtime default)", m["GOFLAGS"])
	b, err := json.Marshal(m)
	require.NoError(t, err)
	require.NotContains(t, string(b), "confidential-sentinel")
	require.NotContains(t, string(b), "GITHUB_TOKEN")
	require.NotContains(t, string(b), "BUNDLE_GEMS__EXAMPLE__COM")
}

// This fixture exercises the report's acceptance boundary, not executable
// forwarding. Actual binaries are exercised separately by the native smoke run.
func reportFixture(t *testing.T) string {
	root := workspace(t)
	r := map[string]string{"schema_version": "1", "source_original": "/original/site", "source": filepath.Join(root, "source"), "site_argument": "small", "runs": "4", "warmup": "0", "profile": "default", "version": "test", "build_date": "2026-11-07", "bundle_gemfile": "/locked/Gemfile", "bundle_path": "/prepared/bundle", "jekyll_disk_cache": filepath.Join(root, "source/.jekyll-cache")}
	var handoff strings.Builder
	for _, key := range []string{"schema_version", "source_original", "source", "site_argument", "runs", "warmup", "profile", "version", "build_date", "bundle_gemfile", "bundle_path", "jekyll_disk_cache"} {
		fmt.Fprintf(&handoff, "%s\t%s\n", key, r[key])
	}
	put(t, filepath.Join(root, "run.tsv"), handoff.String())
	var commands strings.Builder
	commands.WriteString("name\tcommand\tdestination\ttmpdir\txdg_cache_home\tGOMAXPROCS\tJIGYLL_DISABLE_CACHE\n")
	for _, name := range selectedEngines("default") {
		fmt.Fprintf(&commands, "%s\t%s\t%s\t%s\t%s\tdefault\t0\n", name, filepath.Join(root, "commands", name), filepath.Join(root, "output", name), filepath.Join(root, "cache", name, "tmp"), filepath.Join(root, "cache", name, "xdg"))
		put(t, filepath.Join(root, "commands", name), "#!/bin/bash\nexec real-engine build\n")
	}
	put(t, filepath.Join(root, "commands.tsv"), commands.String())
	cs, err := readCommands(root, "default")
	require.NoError(t, err)
	m := runMetadata{Schema: 1, Outcome: "not completed; no valid comparison", Started: time.Now().UTC().Format(time.RFC3339Nano), Run: r, Commands: cs, SourceHashes: map[string]string{"index.html": digest([]byte("input"))}, InputHashes: map[string]string{}, Host: map[string]string{}, Tools: map[string]string{}, Checkout: map[string]string{}, Environment: relevantEnvironment(os.Getenv)}
	for _, key := range []string{"os", "architecture", "cpu_model", "logical_processors", "load_before", "kernel", "destination_filesystem", "destination_root"} {
		m.Host[key] = "recorded"
	}
	for _, key := range []string{"go", "ruby", "bundler", "jekyll", "sass_jigyll", "sass_jekyll_gem", "hyperfine", "jigyll"} {
		m.Tools[key] = "recorded version"
	}
	for _, key := range []string{"commit", "dirty", "status_sha256", "tracked_diff_sha256", "untracked_content_sha256", "untracked_file_count"} {
		m.Checkout[key] = "recorded"
	}
	for _, key := range []string{"binary", "shared_config", "gemfile", "lockfile", "generator_source", "go_mod", "go_sum"} {
		m.InputHashes[key] = digest([]byte(key))
	}
	require.NoError(t, saveJSON(filepath.Join(root, "metadata.json"), m))
	put(t, filepath.Join(root, "source/index.html"), "input")
	put(t, filepath.Join(root, "hyperfine.json"), string(sampleJSON(`[1,2,3,4]`, `[2,4,6,8]`)))
	put(t, filepath.Join(root, "hyperfine.md"), "raw markdown export")
	put(t, filepath.Join(root, "hyperfine-command"), "hyperfine --runs 4 --warmup 0\n")
	put(t, filepath.Join(root, "verification.log"), "verified equivalent work")
	put(t, filepath.Join(root, "preflight-reset"), "reset complete")
	return root
}

func TestReportPreservesSamplesAndZeroWarmupMeaning(t *testing.T) {
	root := reportFixture(t)
	require.NoError(t, writeReport(root))
	var stats []sampleStats
	b, err := os.ReadFile(filepath.Join(root, "statistics.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &stats))
	require.Equal(t, []float64{2, 4, 6, 8}, stats[1].Times)
	require.Equal(t, 0.5, *stats[1].Ratio)
	var m runMetadata
	b, err = os.ReadFile(filepath.Join(root, "metadata.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &m))
	require.Equal(t, "success", m.Outcome)
	require.NotEmpty(t, m.Host["load_after"])
	require.Equal(t, digest([]byte(string(sampleJSON(`[1,2,3,4]`, `[2,4,6,8]`)))), m.ArtifactHashes["hyperfine.json"])
	require.NotEmpty(t, m.Finished)
	b, err = os.ReadFile(filepath.Join(root, "report.md"))
	require.NoError(t, err)
	require.Contains(t, string(b), cacheLabel(0))
	require.NotContains(t, cacheLabel(0), "warm-cache")
	require.Contains(t, cacheLabel(2), "2 warmups")
}

func TestFailedOrIncompleteRunNeverPublishesSuccessReport(t *testing.T) {
	for _, problem := range []string{"failed status", "incomplete status", "failed sample", "missing export", "missing preflight", "missing metadata", "incomplete metadata", "changed source", "bad command handoff", "bad run handoff"} {
		t.Run(problem, func(t *testing.T) {
			root := reportFixture(t)
			switch problem {
			case "failed status":
				put(t, filepath.Join(root, "status"), "failed: hyperfine (exit 1)\n")
			case "incomplete status":
				put(t, filepath.Join(root, "status"), "running\n")
			case "failed sample":
				put(t, filepath.Join(root, "hyperfine.json"), strings.Replace(string(sampleJSON(`[1,2,3,4]`, `[2,4,6,8]`)), `[0,0,0,0]`, `[1,0,0,0]`, 1))
			case "missing export":
				require.NoError(t, os.Remove(filepath.Join(root, "hyperfine.md")))
			case "missing preflight":
				require.NoError(t, os.Remove(filepath.Join(root, "preflight-reset")))
			case "missing metadata":
				require.NoError(t, os.Remove(filepath.Join(root, "metadata.json")))
			case "incomplete metadata":
				put(t, filepath.Join(root, "metadata.json"), `{}`)
			case "changed source":
				put(t, filepath.Join(root, "source/index.html"), "changed")
			case "bad command handoff":
				put(t, filepath.Join(root, "commands.tsv"), "invalid")
			case "bad run handoff":
				put(t, filepath.Join(root, "run.tsv"), "GITHUB_TOKEN\tsecret\n")
			}
			require.Error(t, writeReport(root))
			require.NoFileExists(t, filepath.Join(root, "report.md"))
			require.NoFileExists(t, filepath.Join(root, "statistics.json"))
		})
	}
}

func TestReportWriteFailureDoesNotMarkMetadataSuccessful(t *testing.T) {
	root := reportFixture(t)
	require.NoError(t, os.Mkdir(filepath.Join(root, "report.md"), 0700))
	require.Error(t, writeReport(root))
	b, err := os.ReadFile(filepath.Join(root, "metadata.json"))
	require.NoError(t, err)
	var m runMetadata
	require.NoError(t, json.Unmarshal(b, &m))
	require.Equal(t, "not completed; no valid comparison", m.Outcome)
}
