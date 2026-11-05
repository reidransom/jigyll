package site

import (
	"fmt"
	"net/http"
	"time"

	"github.com/reidransom/jigyll/internal/themearchive"
)

const codeloadGitHubHost = "codeload.github.com"

type remoteThemeResolver struct {
	client     *http.Client
	cacheRoot  string
	archiveURL func(remoteThemeSpec) string
	limits     themearchive.Limits
}

func newRemoteThemeResolver() *remoteThemeResolver {
	return &remoteThemeResolver{
		client: &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: checkCodeloadRedirect,
		},
		archiveURL: remoteThemeArchiveURL,
		limits:     themearchive.DefaultLimits(),
	}
}

func remoteThemeArchiveURL(spec remoteThemeSpec) string {
	return "https://" + codeloadGitHubHost + "/" + spec.Owner + "/" + spec.Repo + "/tar.gz/" + spec.Ref
}

func checkCodeloadRedirect(req *http.Request, _ []*http.Request) error {
	if req.URL.Scheme != "https" || req.URL.Host != codeloadGitHubHost {
		return fmt.Errorf("remote theme redirect must remain on https://%s", codeloadGitHubHost)
	}
	return nil
}
