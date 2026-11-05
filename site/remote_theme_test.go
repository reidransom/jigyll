package site

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRemoteTheme(t *testing.T) {
	const sha = "394d6c0ec33852f8e593145d21344a955e908acb"

	for _, tc := range []struct {
		name string
		raw  string
		want remoteThemeSpec
		err  string
	}{
		{
			name: "pinned GitHub commit",
			raw:  "Just-The-Docs/JUST_the.docs@394D6C0EC33852F8E593145D21344A955E908ACB",
			want: remoteThemeSpec{Owner: "just-the-docs", Repo: "just_the.docs", Ref: sha},
		},
		{name: "missing revision", raw: "owner/repo@", err: "revision is required"},
		{name: "short SHA", raw: "owner/repo@012345678901234567890123456789012345678", err: "40-character hexadecimal"},
		{name: "tag", raw: "owner/repo@v1.2.3", err: "40-character hexadecimal"},
		{name: "branch", raw: "owner/repo@main", err: "40-character hexadecimal"},
		{name: "missing owner", raw: "/repo@" + sha, err: "owner must be"},
		{name: "missing repository", raw: "owner/@" + sha, err: "repository must be"},
		{name: "URL", raw: "https://github.com/owner/repo@" + sha, err: "repository must be owner/repository"},
		{name: "extra path", raw: "owner/repo/extra@" + sha, err: "repository must be owner/repository"},
		{name: "traversal owner", raw: "../repo@" + sha, err: "owner must be a GitHub path segment"},
		{name: "traversal repository", raw: "owner/..@" + sha, err: "repository must be a GitHub path segment"},
		{name: "encoded separator", raw: "owner%2frepo@" + sha, err: "repository must be owner/repository"},
		{name: "query", raw: "owner/repo?x=1@" + sha, err: "repository must be a GitHub path segment"},
		{name: "fragment", raw: "owner/repo#v1@" + sha, err: "repository must be a GitHub path segment"},
		{name: "extra at separator", raw: "owner/repo@" + sha + "@extra", err: "exactly one @"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRemoteTheme(tc.raw)
			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)
				require.ErrorContains(t, err, tc.raw)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestRemoteThemeArchivePolicy(t *testing.T) {
	spec := remoteThemeSpec{Owner: "owner", Repo: "repo", Ref: "0123456789012345678901234567890123456789"}
	require.Equal(t, "https://codeload.github.com/owner/repo/tar.gz/"+spec.Ref, remoteThemeArchiveURL(spec))
	resolver := newRemoteThemeResolver()
	for _, tc := range []struct {
		url     string
		allowed bool
	}{
		{url: "https://codeload.github.com/owner/repo/tar.gz/commit", allowed: true},
		{url: "http://codeload.github.com/archive"},
		{url: "https://github.com/archive"},
		{url: "https://codeload.github.com.evil.example/archive"},
		{url: "https://codeload.github.com:8443/archive"},
	} {
		t.Run(tc.url, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodGet, tc.url, nil)
			require.NoError(t, err)
			err = resolver.client.CheckRedirect(request, nil)
			if tc.allowed {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
