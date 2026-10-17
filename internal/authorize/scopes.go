package authorize

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/donaldgifford/docz/v2/internal/auth"
	"github.com/donaldgifford/docz/v2/internal/session"
	doczcfg "github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// ScopesFor computes a person's scopes (DESIGN-0023 §7): public:read, which
// every principal holds, plus whatever the scope map grants for
// `<provider>:group:<g>` over the identity's groups and for
// `<provider>:login:<login>`, sorted and de-duplicated.
func ScopesFor(identity *auth.Identity, scopeMap map[string][]string) []string {
	set := map[string]struct{}{doczcfg.ScopePublicRead: {}}

	grant := func(key string) {
		for _, s := range scopeMap[key] {
			set[s] = struct{}{}
		}
	}

	for _, g := range identity.Groups {
		grant(identity.Provider + ":group:" + g)
	}
	if identity.Login != "" {
		grant(identity.Provider + ":login:" + identity.Login)
	}

	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	slices.Sort(out)

	return out
}

// ScopeLister is the store surface ScopeAuthorizer needs; *store.Store
// satisfies it.
type ScopeLister interface {
	ListRepoIDsForScopes(ctx context.Context, scopes []string) ([]int64, error)
}

// ScopeAuthorizer resolves the allowed set from the session's scopes: a
// repository is visible when its access_scopes holds any scope the
// principal does. The answer is cached per principal key for the TTL, so
// a burst of requests from one caller is one query (DESIGN-0023 §7, §9).
type ScopeAuthorizer struct {
	lister ScopeLister
	ttl    time.Duration
	now    func() time.Time

	mu    sync.Mutex
	cache map[string]cachedAllowed
}

type cachedAllowed struct {
	allowed AllowedRepos
	scopes  []string
	expires time.Time
}

var _ Authorizer = (*ScopeAuthorizer)(nil)

// NewScopeAuthorizer builds the authorizer; a zero ttl disables the cache.
func NewScopeAuthorizer(lister ScopeLister, ttl time.Duration) *ScopeAuthorizer {
	return &ScopeAuthorizer{
		lister: lister,
		ttl:    ttl,
		now:    time.Now,
		cache:  make(map[string]cachedAllowed),
	}
}

// Allowed returns the repositories the session in ctx may read. A request
// with no session, which the gate never produces, sees nothing.
func (a *ScopeAuthorizer) Allowed(ctx context.Context, _ *http.Request) (AllowedRepos, error) {
	sess, ok := session.FromContext(ctx)
	if !ok || len(sess.Scopes) == 0 {
		return AllowedRepos{}, nil
	}

	key := sess.PrincipalKey()
	if allowed, hit := a.lookup(key, sess.Scopes); hit {
		return allowed, nil
	}

	ids, err := a.lister.ListRepoIDsForScopes(ctx, sess.Scopes)
	if err != nil {
		return nil, fmt.Errorf("list repos for scopes: %w", err)
	}
	allowed := AllowedRepos(ids)
	if allowed == nil {
		allowed = AllowedRepos{}
	}
	a.remember(key, sess.Scopes, allowed)

	return allowed, nil
}

// lookup returns the cached set when it is fresh and was computed for the
// same scopes; a principal whose scopes changed within the TTL (a group
// granted mid-session, a token with a narrower scope) is not served the old
// answer.
func (a *ScopeAuthorizer) lookup(key string, scopes []string) (AllowedRepos, bool) {
	if a.ttl <= 0 {
		return nil, false
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	entry, ok := a.cache[key]
	if !ok || !a.now().Before(entry.expires) || !slices.Equal(entry.scopes, scopes) {
		delete(a.cache, key)

		return nil, false
	}

	return entry.allowed, true
}

func (a *ScopeAuthorizer) remember(key string, scopes []string, allowed AllowedRepos) {
	if a.ttl <= 0 {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.cache[key] = cachedAllowed{
		allowed: allowed,
		scopes:  slices.Clone(scopes),
		expires: a.now().Add(a.ttl),
	}
}

// Forget drops a principal's cached set, for a test or an operator action.
func (a *ScopeAuthorizer) Forget(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.cache, key)
}
