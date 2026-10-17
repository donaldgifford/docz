package oauth

import (
	"strings"
	"time"
)

// Claims is what the verifier hands back for an accepted token: the subset
// of the JWT docz-api acts on, with the scopes already split.
type Claims struct {
	Issuer   string
	Subject  string
	ClientID string
	Scopes   []string
	Email    string
	Login    string
	Expiry   time.Time
	// ID is the jti, when the issuer set one.
	ID string
}

// SubjectPrefix is the prefix docz-api puts on the sub of a token it mints:
// `user:<provider>:<subject>`, so a cookie and a token for one person
// resolve to one principal.
const SubjectPrefix = "user:"

// UserSubject splits a docz-api subject into the login provider and the
// provider's subject. ok is false for a subject docz-api did not mint.
func UserSubject(sub string) (provider, subject string, ok bool) {
	rest, found := strings.CutPrefix(sub, SubjectPrefix)
	if !found {
		return "", "", false
	}

	provider, subject, ok = strings.Cut(rest, ":")
	if !ok || provider == "" || subject == "" {
		return "", "", false
	}

	return provider, subject, true
}

// claimSet is the wire shape of the claims docz-api reads beyond the
// registered ones go-jose validates.
type claimSet struct {
	ClientID string `json:"client_id,omitempty"`
	Email    string `json:"email,omitempty"`
	Login    string `json:"login,omitempty"`
	// Scope is the space-separated RFC 8693 form; Scp the array form some
	// issuers (Okta) use instead.
	Scope string   `json:"scope,omitempty"`
	Scp   []string `json:"scp,omitempty"`
}

// scopes returns the token's scopes from whichever claim carries them,
// trimmed, with empties and duplicates dropped and order kept.
func (c *claimSet) scopes() []string {
	var raw []string
	if c.Scope != "" {
		raw = strings.Fields(c.Scope)
	} else {
		raw = c.Scp
	}

	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	return out
}
