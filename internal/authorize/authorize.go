// Package authorize is the authorization seam for docz-api's read endpoints.
//
// A Middleware resolves the set of repos the current request may read (via an
// Authorizer) and injects it into the request context; handlers read it back
// with FromContext and use it for existence-hiding (404 for repos not in the
// set). Phase 2 ships AllReposAuthorizer, which grants every onboarded repo;
// Phase 5 swaps in a per-user implementation behind the same Authorizer
// interface without touching handlers.
package authorize

import (
	"context"
	"net/http"
	"slices"
)

// AllowedRepos is the set of repo IDs the current request may read. A nil or
// empty value means no repos are visible.
type AllowedRepos []int64

// Contains reports whether the repo id is in the allowed set.
func (a AllowedRepos) Contains(id int64) bool {
	return slices.Contains(a, id)
}

// ctxKey is the unexported context key type for the allowed-repo set.
type ctxKey struct{}

// FromContext returns the AllowedRepos injected by Middleware, or nil if the
// request did not pass through it.
func FromContext(ctx context.Context) AllowedRepos {
	v, ok := ctx.Value(ctxKey{}).(AllowedRepos)
	if !ok {
		return nil
	}
	return v
}

// Authorizer resolves the allowed-repo set for one request.
type Authorizer interface {
	Allowed(ctx context.Context, r *http.Request) (AllowedRepos, error)
}

// Middleware resolves the allowed-repo set with a and injects it into the
// request context. An authorizer error yields 500; the injected set may be
// empty, which handlers treat as "no access".
func Middleware(a Authorizer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowed, err := a.Allowed(r.Context(), r)
			if err != nil {
				http.Error(w, `{"error":"authorization failed"}`, http.StatusInternalServerError)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, allowed)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
