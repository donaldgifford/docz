package githubapp

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-github/v88/github"
)

// selfCheckTransport serves a canned response for GET /app.
type selfCheckTransport struct {
	status int
	body   string
	err    error
}

func (s selfCheckTransport) RoundTrip(*http.Request) (*http.Response, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Header:     jsonHeader(),
	}, nil
}

func newSelfCheckClient(t *testing.T, tr http.RoundTripper) *github.Client {
	t.Helper()
	gh, err := github.NewClient(github.WithTransport(tr))
	if err != nil {
		t.Fatalf("new github client: %v", err)
	}
	return gh
}

func TestSelfCheckReturnsAppIdentity(t *testing.T) {
	gh := newSelfCheckClient(t, selfCheckTransport{
		status: http.StatusOK,
		body:   `{"id":12345,"slug":"docz-api","name":"docz API"}`,
	})

	app, err := selfCheck(t.Context(), gh)
	if err != nil {
		t.Fatalf("selfCheck: %v", err)
	}
	if app.ID != 12345 {
		t.Errorf("ID = %d, want 12345", app.ID)
	}
	if app.Slug != "docz-api" {
		t.Errorf("Slug = %q, want %q", app.Slug, "docz-api")
	}
	if app.Name != "docz API" {
		t.Errorf("Name = %q, want %q", app.Name, "docz API")
	}
}

// A 401 is GitHub refusing the credentials: permanent, so it must be
// distinguishable and fail startup.
func TestSelfCheckClassifiesRejection(t *testing.T) {
	gh := newSelfCheckClient(t, selfCheckTransport{
		status: http.StatusUnauthorized,
		body:   `{"message":"A JSON web token could not be decoded"}`,
	})

	_, err := selfCheck(t.Context(), gh)
	if err == nil {
		t.Fatal("selfCheck succeeded, want an error")
	}
	if !errors.Is(err, ErrCredentialsRejected) {
		t.Errorf("err = %v, want it to wrap ErrCredentialsRejected", err)
	}
}

// An unreachable GitHub is transient and must NOT be mistaken for bad
// credentials, or a GitHub outage during a restart becomes a crash loop.
func TestSelfCheckTransportErrorIsNotARejection(t *testing.T) {
	gh := newSelfCheckClient(t, selfCheckTransport{err: errors.New("connection refused")})

	_, err := selfCheck(t.Context(), gh)
	if err == nil {
		t.Fatal("selfCheck succeeded, want an error")
	}
	if errors.Is(err, ErrCredentialsRejected) {
		t.Errorf("err = %v, want it NOT to be classified as a credential rejection", err)
	}
}

// Rate limiting also arrives as a 4xx but is transient, so it must fall on the
// non-fatal side of the split.
func TestSelfCheckRateLimitIsNotARejection(t *testing.T) {
	header := jsonHeader()
	header.Set("X-RateLimit-Remaining", "0")
	header.Set("X-RateLimit-Limit", "60")
	header.Set("X-RateLimit-Reset", "1750000000")

	gh := newSelfCheckClient(t, roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Body: io.NopCloser(strings.NewReader(
				`{"message":"API rate limit exceeded","documentation_url":"https://docs.github.com/"}`)),
			Header: header,
		}, nil
	}))

	_, err := selfCheck(t.Context(), gh)
	if err == nil {
		t.Fatal("selfCheck succeeded, want an error")
	}
	if errors.Is(err, ErrCredentialsRejected) {
		t.Errorf("err = %v, want rate limiting NOT classified as a credential rejection", err)
	}
}

// roundTripperFunc adapts a function to http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// A malformed private key fails when the app transport is built, before any
// request — still a credential fault, and the most common one when moving
// between secret stores.
func TestSelfCheckRejectsMalformedPrivateKey(t *testing.T) {
	_, err := SelfCheck(t.Context(), 1, []byte("not a pem key"), "")
	if err == nil {
		t.Fatal("SelfCheck succeeded with a malformed key, want an error")
	}
	if !errors.Is(err, ErrCredentialsRejected) {
		t.Errorf("err = %v, want it to wrap ErrCredentialsRejected", err)
	}
}

// GitHub uses 403 for primary rate limiting as well as for a suspended
// installation, and go-github only types a *RateLimitError when the response
// carries X-RateLimit-Remaining: 0 — so a 403 from a proxy, a CDN, or a GHES
// front end is indistinguishable from a refusal. Classifying any of these as
// permanent would crash-loop a deploy whose key is perfectly good, which is the
// exact outcome SelfCheck exists to prevent.
func TestSelfCheckOnlyTreats401AsRejection(t *testing.T) {
	tests := []struct {
		status   int
		rejected bool
	}{
		{http.StatusUnauthorized, true},
		{http.StatusForbidden, false},
		{http.StatusRequestTimeout, false},
		{http.StatusTooManyRequests, false},
		{http.StatusNotFound, false},
		{http.StatusInternalServerError, false},
	}
	for _, tc := range tests {
		// No rate-limit headers and no abuse documentation_url, so go-github
		// yields a plain *ErrorResponse for every one of these.
		gh := newSelfCheckClient(t, selfCheckTransport{
			status: tc.status,
			body:   `{"message":"denied"}`,
		})

		_, err := selfCheck(t.Context(), gh)
		if err == nil {
			t.Fatalf("status %d: selfCheck succeeded, want an error", tc.status)
		}
		if got := errors.Is(err, ErrCredentialsRejected); got != tc.rejected {
			t.Errorf("status %d: rejected = %v, want %v", tc.status, got, tc.rejected)
		}
	}
}
