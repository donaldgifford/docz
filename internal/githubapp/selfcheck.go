package githubapp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v88/github"
)

// ErrCredentialsRejected marks a self-check failure caused by the App
// credentials themselves — a malformed private key, or an app id and key that
// GitHub refuses. Such a failure is permanent: no amount of retrying fixes it,
// so callers should fail startup rather than serve with an App that can never
// ingest.
//
// Failures that are *not* this (DNS, refused connections, timeouts, rate
// limits) are transient and must not take down an otherwise healthy API.
var ErrCredentialsRejected = errors.New("github app credentials rejected")

// AppIdentity is what a successful self-check learned about the authenticated
// App. Logging it at startup turns "which App am I?" into an observable fact
// rather than an assumption about which secret got mounted.
type AppIdentity struct {
	ID   int64
	Slug string
	Name string
}

// SelfCheck authenticates as the App itself and reads back its own identity.
//
// It exists because the App credentials are otherwise exercised nowhere before
// the first ingest job: webhooks prove only that GitHub can reach us, and
// /readyz deliberately checks serving dependencies only (GitHub being down must
// not pull the read API out of rotation). Without this, a mangled private key
// surfaced as retries that failed silently and then blocked the repo
// (INV-0007 F5). Here it surfaces once, at boot, naming the cause.
//
// Unlike NewClient this uses an *app* JWT rather than an installation token, so
// it needs no installation id and can run before any repo is onboarded.
func SelfCheck(ctx context.Context, appID int64, pemKey []byte, apiBase string) (*AppIdentity, error) {
	// NewAppsTransport parses the PEM, so a malformed key fails here rather than
	// at the API call — still a credential fault, and the most common one when
	// moving between secret stores.
	atr, err := ghinstallation.NewAppsTransport(http.DefaultTransport, appID, pemKey)
	if err != nil {
		return nil, fmt.Errorf("%w: build app transport: %w", ErrCredentialsRejected, err)
	}

	opts := []github.ClientOptionsFunc{github.WithTransport(atr)}
	if apiBase != "" && apiBase != defaultAPIBase {
		base := strings.TrimSuffix(apiBase, "/")
		atr.BaseURL = base
		opts = append(opts, github.WithEnterpriseURLs(base, base))
	}

	gh, err := github.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("build github client: %w", err)
	}
	return selfCheck(ctx, gh)
}

// selfCheck performs the identity call against an already-built client, so the
// classification of GitHub's answer is testable with a stub transport.
func selfCheck(ctx context.Context, gh *github.Client) (*AppIdentity, error) {
	// An empty slug means "the app this JWT authenticates as" (GET /app).
	app, _, err := gh.Apps.Get(ctx, "")
	if err != nil {
		if isCredentialRejection(err) {
			return nil, fmt.Errorf("%w: %w", ErrCredentialsRejected, err)
		}
		return nil, fmt.Errorf("reach github to authenticate the app: %w", err)
	}
	return &AppIdentity{ID: app.GetID(), Slug: app.GetSlug(), Name: app.GetName()}, nil
}

// isCredentialRejection reports whether GitHub refused the credentials, as
// opposed to being unreachable, throttling us, or refusing for some other
// operational reason.
//
// Only 401 counts. SelfCheck calls GET /app, which authenticates the App itself
// with a JWT signed by the private key and needs no permissions at all, so a
// bad key is the only thing 401 can mean there.
//
// Everything else is deliberately treated as transient, because the two
// misclassifications are not symmetric. Calling a transient failure permanent
// crash-loops a deploy whose credentials are fine, taking down the read API for
// a problem that only affects ingest. Calling a permanent failure transient
// costs one startup warning — and since INV-0007 every ingest attempt now logs
// its own cause, so a genuinely bad key is still obvious within one job.
//
// 403 is the case that forces the issue: GitHub uses it for primary rate
// limiting as well as for suspended installations, and go-github only produces
// a typed *RateLimitError when the response carries X-RateLimit-Remaining: 0.
// A 403 from a proxy, a CDN, or a GHES front end therefore arrives as a plain
// *ErrorResponse and is indistinguishable from a real refusal.
func isCredentialRejection(err error) bool {
	var apiErr *github.ErrorResponse
	if !errors.As(err, &apiErr) || apiErr.Response == nil {
		return false
	}
	return apiErr.Response.StatusCode == http.StatusUnauthorized
}
