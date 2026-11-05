package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/reidransom/jigyll/internal/themearchive"
	"github.com/stretchr/testify/require"
)

const referenceCommit = "0123456789abcdef0123456789abcdef01234567"
const referenceTagObject = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestParseThemeReference(t *testing.T) {
	for _, tc := range []struct {
		raw, repository, name, ref string
		github                     bool
	}{
		{"owner/docifly@latest", "https://github.com/owner/docifly", "docifly", "latest", true},
		{"owner/Doc.Theme-1_2@v1.2.3", "https://github.com/owner/Doc.Theme-1_2", "Doc.Theme-1_2", "v1.2.3", true},
		{"owner/docifly@" + referenceCommit, "https://github.com/owner/docifly", "docifly", referenceCommit, true},
		{"https://github.com/owner/docifly@v1", "https://github.com/owner/docifly", "docifly", "v1", true},
		{"https://src.r2ware.dev/owner/docifly@v1.2.3", "https://src.r2ware.dev/owner/docifly", "docifly", "v1.2.3", false},
		{"http://127.0.0.1:3456/owner/docifly@release/v1", "http://127.0.0.1:3456/owner/docifly", "docifly", "release/v1", false},
		{"http://localhost:1234/owner/docifly.git@v1", "http://localhost:1234/owner/docifly", "docifly", "v1", false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			ref, recognized, err := parseThemeReference(tc.raw)
			require.NoError(t, err)
			require.True(t, recognized)
			require.Equal(t, tc.raw, ref.Source)
			require.Equal(t, tc.repository, ref.RepositoryURL)
			require.Equal(t, tc.name, ref.Name)
			require.Equal(t, tc.ref, ref.Reference)
			require.Equal(t, tc.github, ref.GitHub)
		})
	}
}

func TestParseThemeReferencePreservesGitForms(t *testing.T) {
	local := filepath.Join(t.TempDir(), "theme@v1")
	require.NoError(t, os.Mkdir(local, 0o755))
	for _, raw := range []string{
		"", "git@github.com:owner/docifly.git", "github.com:owner/docifly.git", "git@forge:owner/docifly@v1.git",
		"ssh://git@github.com/owner/docifly.git", "ssh://git@forge/owner/docifly@v1.git", "git://github.com/owner/docifly.git",
		"https://github.com/owner/docifly.git", "https://user@host/owner/docifly.git", "http://localhost:1234/owner/docifly",
		"file:///tmp/theme@v1", "file:theme@v1", "/tmp/theme@v1", "./owner/docifly@v1", "../owner/docifly@v1", "~/themes/docifly@v1",
		`C:\themes\docifly@v1`, "owner/docifly.git", "some/local/theme", local,
	} {
		t.Run(raw, func(t *testing.T) {
			_, recognized, err := parseThemeReference(raw)
			require.NoError(t, err)
			require.False(t, recognized)
		})
	}
	// An existing relative path takes precedence even over shorthand syntax.
	parent, err := os.MkdirTemp(".", "reference-local-")
	require.NoError(t, err)
	defer os.RemoveAll(parent)
	relative := filepath.Join(parent, "docifly@v1")
	require.NoError(t, os.Mkdir(relative, 0o755))
	_, recognized, err := parseThemeReference(relative)
	require.NoError(t, err)
	require.False(t, recognized)
}

func TestParseThemeReferenceRejectsMalformedSelections(t *testing.T) {
	for _, raw := range []string{
		"owner/docifly", "owner/docifly@", "owner/docifly@@v1", "owner/docifly@v 1", "owner/docifly@refs//v1",
		"owner/docifly@../main", "owner/docifly@v1.lock", "owner/docifly@v1~1", "owner/docifly@v1?x",
		"owner/.@v1", "owner/..@v1", "owner/.git@v1", "owner/bad name@v1", "owner@latest", "owner/repo/extra@v1",
		"https://host/owner/docifly@", "https://host/owner/docifly@v1?x=1", "https://host/owner/docifly@v1#x",
		"https://user@host/owner/docifly@v1", "https://host/owner/%2e%2e@v1", "https://host/owner/repo%2Fextra@v1",
		"http:///owner/docifly@v1", "https://host:bad/owner/docifly@v1",
		"https:owner/docifly@v1", "http:owner/docifly@v1",
		`owner/docifly@bad\ref`,
	} {
		t.Run(raw, func(t *testing.T) {
			_, recognized, err := parseThemeReference(raw)
			require.True(t, recognized)
			require.ErrorContains(t, err, "invalid theme repository reference")
		})
	}
}

type referenceTransport func(*http.Request) (*http.Response, error)

func (fn referenceTransport) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

// GitHub's fixed origins remain production constants; only the transport is
// redirected to a local provider-shaped HTTP fixture.
func referenceClient(server *httptest.Server) *http.Client {
	origin, _ := url.Parse(server.URL)
	return themearchive.NewHTTPClient(referenceTransport(func(req *http.Request) (*http.Response, error) {
		clone := req.Clone(req.Context())
		u := *req.URL
		u.Scheme, u.Host = origin.Scheme, origin.Host
		clone.URL = &u
		return server.Client().Transport.RoundTrip(clone)
	}))
}

func TestResolveThemeReferenceProviders(t *testing.T) {
	for _, github := range []bool{true, false} {
		for _, selection := range []string{"latest", "v1.2.3", "release/v1", referenceCommit, strings.ToUpper(referenceCommit)} {
			for _, annotated := range []bool{false, true} {
				t.Run(fmt.Sprintf("github=%v/%s/annotated=%v", github, selection, annotated), func(t *testing.T) {
					var requests []string
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requests = append(requests, r.URL.Path)
						switch {
						case strings.HasSuffix(r.URL.Path, "/releases/latest"):
							io.WriteString(w, `{"tag_name":"v1.2.3","draft":false,"prerelease":false}`)
						case strings.Contains(r.URL.Path, "/git/ref/tags/"):
							tag := strings.SplitN(r.URL.Path, "/git/ref/tags/", 2)[1]
							kind, sha := "commit", referenceCommit
							if annotated {
								kind, sha = "tag", referenceTagObject
							}
							json.NewEncoder(w).Encode(map[string]any{"ref": "refs/tags/" + tag, "object": map[string]string{"type": kind, "sha": sha}})
						case strings.Contains(r.URL.Path, "/git/tags/"):
							fmt.Fprintf(w, `{"object":{"type":"commit","sha":%q}}`, referenceCommit)
						case strings.Contains(r.URL.Path, "/tags/"):
							tag := strings.SplitN(r.URL.Path, "/tags/", 2)[1]
							id := referenceCommit
							if annotated {
								id = referenceTagObject
							}
							fmt.Fprintf(w, `{"name":%q,"id":%q,"commit":{"sha":%q}}`, tag, id, referenceCommit)
						case strings.Contains(r.URL.Path, "/git/commits/"):
							fmt.Fprintf(w, `{"sha":%q}`, referenceCommit)
						default:
							http.Error(w, "unexpected request", http.StatusNotFound)
						}
					}))
					defer server.Close()
					source := "owner/docifly@" + selection
					if !github {
						source = server.URL + "/" + source
					}
					ref, recognized, err := parseThemeReference(source)
					require.NoError(t, err)
					require.True(t, recognized)
					resolved, err := resolveThemeReference(context.Background(), referenceClient(server), ref)
					require.NoError(t, err)
					require.Equal(t, source, resolved.Source)
					require.Equal(t, ref.RepositoryURL, resolved.RepositoryURL)
					require.Equal(t, "docifly", resolved.Name)
					require.Equal(t, referenceCommit, resolved.ResolvedCommit)
					expectedTag := selection
					if selection == "latest" {
						expectedTag = "v1.2.3"
					}
					if themeCommitSHA.MatchString(selection) {
						expectedTag = ""
					}
					require.Equal(t, expectedTag, resolved.ResolvedTag)
					if github {
						require.Equal(t, "https://codeload.github.com/owner/docifly/tar.gz/"+referenceCommit, resolved.ArchiveURL)
					} else {
						require.Equal(t, server.URL+"/api/v1/repos/owner/docifly/archive/"+referenceCommit+".tar.gz", resolved.ArchiveURL)
					}
					for _, path := range requests {
						require.NotContains(t, path, "/archive/")
					}
				})
			}
		}
	}
}

func TestResolveThemeReferenceImmutableArchive(t *testing.T) {
	for _, github := range []bool{true, false} {
		t.Run(fmt.Sprint(github), func(t *testing.T) {
			commit := referenceCommit
			archiveRequests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.Contains(r.URL.Path, "/git/ref/tags/"):
					fmt.Fprintf(w, `{"ref":"refs/tags/v1","object":{"type":"commit","sha":%q}}`, commit)
				case strings.Contains(r.URL.Path, "/tags/"):
					fmt.Fprintf(w, `{"name":"v1","commit":{"sha":%q}}`, commit)
				case strings.HasSuffix(r.URL.Path, referenceCommit), strings.HasSuffix(r.URL.Path, referenceCommit+".tar.gz"):
					archiveRequests++
					io.WriteString(w, "archive for original commit")
				default:
					http.Error(w, "wrong revision", http.StatusNotFound)
				}
			}))
			defer server.Close()
			raw := "owner/docifly@v1"
			if !github {
				raw = server.URL + "/" + raw
			}
			ref, _, err := parseThemeReference(raw)
			require.NoError(t, err)
			client := referenceClient(server)
			resolved, err := resolveThemeReference(context.Background(), client, ref)
			require.NoError(t, err)
			commit = strings.Repeat("a", 40) // The mutable tag moves after metadata resolution.
			file := filepath.Join(t.TempDir(), "archive.tar.gz")
			_, err = themearchive.Download(context.Background(), client, resolved.ArchiveURL, file, themearchive.DefaultLimits())
			require.NoError(t, err)
			body, err := os.ReadFile(file)
			require.NoError(t, err)
			require.Equal(t, "archive for original commit", string(body))
			require.Equal(t, 1, archiveRequests)
		})
	}
}

func TestResolveThemeReferenceFailures(t *testing.T) {
	for _, github := range []bool{true, false} {
		for _, tc := range []struct {
			name, selection, body, want string
			status                      int
		}{
			{"missing latest", "latest", "", "latest published theme release", 404},
			{"prerelease only", "latest", `{"tag_name":"v2-rc1","prerelease":true}`, "not a published", 200},
			{"draft", "latest", `{"tag_name":"v2","draft":true}`, "not a published", 200},
			{"empty release", "latest", `{}`, "not a published", 200},
			{"bad release tag", "latest", `{"tag_name":"../v1"}`, "not a published", 200},
			{"missing tag", "missing", "", "exact theme tag", 404},
			{"branch not tag", "main", "", "exact theme tag", 404},
			{"missing repository", "v1", "", "HTTP 200", 404},
			{"server error", "v1", "", "500", 500},
			{"missing commit", referenceCommit, "", "theme commit", 404},
			{"wrong commit", referenceCommit, `{"sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, "does not match", 200},
			{"malformed", "v1", `{`, "decode theme metadata", 200},
			{"null", "v1", `null`, "requested exact tag", 200},
			{"multiple JSON", "v1", `{} {}`, "single JSON", 200},
			{"trailing garbage", "v1", `{} garbage`, "single JSON", 200},
			{"large metadata", "v1", `{}` + strings.Repeat(" ", 1<<20), "exceeds 1 MiB", 200},
		} {
			t.Run(fmt.Sprintf("github=%v/%s", github, tc.name), func(t *testing.T) {
				requests := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests++
					w.WriteHeader(tc.status)
					io.WriteString(w, tc.body)
				}))
				defer server.Close()
				raw := "owner/docifly@" + tc.selection
				if !github {
					raw = server.URL + "/" + raw
				}
				ref, _, err := parseThemeReference(raw)
				require.NoError(t, err)
				_, err = resolveThemeReference(context.Background(), referenceClient(server), ref)
				require.ErrorContains(t, err, tc.want)
				require.Equal(t, 1, requests, "failures must not select another version or retry")
			})
		}
	}
}

func TestResolveThemeReferenceInvalidTagObjects(t *testing.T) {
	for _, tc := range []struct{ name, github, forgejo, want string }{
		{"invalid SHA", `{"ref":"refs/tags/v1","object":{"type":"commit","sha":"short"}}`, `{"name":"v1","commit":{"sha":"short"}}`, "invalid full commit SHA"},
		{"wrong tag", `{"ref":"refs/tags/other","object":{"type":"commit","sha":"` + referenceCommit + `"}}`, `{"name":"other","commit":{"sha":"` + referenceCommit + `"}}`, "requested exact tag"},
		{"branch response", `{"ref":"refs/heads/v1","object":{"type":"commit","sha":"` + referenceCommit + `"}}`, `{"name":"main","commit":{"sha":"` + referenceCommit + `"}}`, "requested exact tag"},
	} {
		for _, github := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/%v", tc.name, github), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if github {
						io.WriteString(w, tc.github)
					} else {
						io.WriteString(w, tc.forgejo)
					}
				}))
				defer server.Close()
				raw := "owner/docifly@v1"
				if !github {
					raw = server.URL + "/" + raw
				}
				ref, _, err := parseThemeReference(raw)
				require.NoError(t, err)
				_, err = resolveThemeReference(context.Background(), referenceClient(server), ref)
				require.ErrorContains(t, err, tc.want)
			})
		}
	}
	for _, kind := range []string{"tree", "", "tag"} {
		t.Run("annotated/"+kind, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/git/ref/") {
					fmt.Fprintf(w, `{"ref":"refs/tags/v1","object":{"type":"tag","sha":%q}}`, referenceTagObject)
				} else {
					fmt.Fprintf(w, `{"object":{"type":%q,"sha":%q}}`, kind, referenceTagObject)
				}
			}))
			defer server.Close()
			ref, _, err := parseThemeReference("owner/docifly@v1")
			require.NoError(t, err)
			_, err = resolveThemeReference(context.Background(), referenceClient(server), ref)
			require.Error(t, err)
		})
	}
}

func TestResolveThemeReferenceNetworkFailures(t *testing.T) {
	for _, disconnect := range []bool{true, false} {
		t.Run(fmt.Sprint(disconnect), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if disconnect {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err == nil {
						conn.Close()
					}
				} else {
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			ref, _, err := parseThemeReference(server.URL + "/owner/docifly@latest")
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			_, err = resolveThemeReference(ctx, themearchive.NewHTTPClient(nil), ref)
			require.ErrorContains(t, err, "request theme metadata")
			if !disconnect {
				require.ErrorIs(t, err, context.DeadlineExceeded)
			}
		})
	}
}
