package oauth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/donaldgifford/docz/v2/internal/auth"
	"github.com/donaldgifford/docz/v2/internal/store"
	doczcfg "github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// The grants the token endpoint serves.
const (
	GrantAuthorizationCode = "authorization_code"
	GrantRefreshToken      = "refresh_token"
)

// Descriptions the token endpoint repeats.
const (
	errMint   = "could not mint a token"
	errReused = "refresh_token was already used; its family is revoked"
)

// TokenResponse is RFC 6749 §5.1's success body.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// token is POST /oauth/token.
func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, ErrorInvalidRequest, "the body must be application/x-www-form-urlencoded")

		return
	}

	grant := r.PostForm.Get("grant_type")
	var (
		resp TokenResponse
		err  *tokenError
	)
	switch grant {
	case GrantAuthorizationCode:
		resp, err = s.redeemCode(r.Context(), r.PostForm)
	case GrantRefreshToken:
		resp, err = s.refresh(r.Context(), r.PostForm)
	default:
		err = &tokenError{code: ErrorUnsupportedGrant, description: "grant_type must be authorization_code or refresh_token"}
	}

	if err != nil {
		s.observe(grant, "rejected")
		audit(r.Context(), grant, r.PostForm.Get("client_id"), err.principal, err.code)
		writeOAuthError(w, err.code, err.description)

		return
	}

	s.observe(grant, "ok")
	writeJSON(w, http.StatusOK, resp)
}

// tokenError is a refusal with the principal, when one was identified, for
// the audit line.
type tokenError struct {
	code        string
	description string
	principal   string
}

// redeemCode is the authorization_code grant: the code once, PKCE, the
// client and redirect_uri and resource as requested, then an access token
// and a new refresh family. A second redemption revokes that family.
func (s *Server) redeemCode(ctx context.Context, form map[string][]string) (TokenResponse, *tokenError) {
	get := formGetter(form)
	raw := get(paramCode)
	if raw == "" {
		return TokenResponse{}, &tokenError{code: ErrorInvalidRequest, description: "code is required"}
	}

	key := hashToken(raw)
	if family, err := s.cache.Take(ctx, redeemedPrefix+key); err == nil {
		// OAuth 2.1 §4.1.3: a replayed code revokes everything it issued.
		s.revokeFamily(ctx, string(family))

		return TokenResponse{}, &tokenError{code: ErrorInvalidGrant, description: "code was already redeemed; its tokens are revoked"}
	}

	stored, err := s.cache.Take(ctx, codePrefix+key)
	if err != nil {
		return TokenResponse{}, &tokenError{code: ErrorInvalidGrant, description: "code is unknown or expired"}
	}
	var c code
	if json.Unmarshal(stored, &c) != nil {
		return TokenResponse{}, &tokenError{code: ErrorInvalidGrant, description: "code is unknown or expired"}
	}
	principal := SubjectPrefix + c.Identity.Provider + ":" + c.Identity.Subject

	if terr := s.checkCodeRequest(get, &c.Pending); terr != nil {
		terr.principal = principal

		return TokenResponse{}, terr
	}

	refresh, terr := s.newFamily(ctx, &c, key)
	if terr != nil {
		terr.principal = principal

		return TokenResponse{}, terr
	}

	resp, terr := s.mint(&c.Identity, c.Pending.ClientID, c.Granted, refresh)
	if terr != nil {
		terr.principal = principal

		return TokenResponse{}, terr
	}
	audit(ctx, GrantAuthorizationCode, c.Pending.ClientID, principal, "ok")

	return resp, nil
}

// checkCodeRequest holds the code's redemption to the authorization it
// came from: PKCE, the client, the redirect_uri, and the resource.
func (s *Server) checkCodeRequest(get func(string) string, p *pending) *tokenError {
	verifier := get("code_verifier")
	if !validPKCE(verifier) || subtle.ConstantTimeCompare([]byte(s256(verifier)), []byte(p.CodeChallenge)) != 1 {
		return &tokenError{code: ErrorInvalidGrant, description: "code_verifier does not match the code_challenge"}
	}
	if get("client_id") != p.ClientID {
		return &tokenError{code: ErrorInvalidClient, description: "client_id is not the one the code was issued to"}
	}
	if get("redirect_uri") != p.RedirectURI {
		return &tokenError{code: ErrorInvalidGrant, description: "redirect_uri is not the one the authorization used"}
	}
	if get("resource") != s.audience {
		return &tokenError{code: ErrorInvalidTarget, description: "resource must be " + s.audience}
	}

	return nil
}

// newFamily starts a refresh family for a redeemed code and remembers the
// redemption, so a replay can revoke the family. It returns the refresh
// token, the one time it exists in the clear.
func (s *Server) newFamily(ctx context.Context, c *code, codeKey string) (string, *tokenError) {
	family, err := randomSecret()
	if err != nil {
		return "", &tokenError{code: ErrorServer, description: errMint}
	}
	refresh, err := randomSecret()
	if err != nil {
		return "", &tokenError{code: ErrorServer, description: errMint}
	}
	if err := s.store.InsertRefreshToken(ctx, hashToken(refresh), family, c.UserID, c.Pending.ClientID, joinScopes(c.Granted)); err != nil {
		slog.ErrorContext(ctx, "oauth token: insert refresh token", "err", err)

		return "", &tokenError{code: ErrorTemporarilyUnavail, description: "could not store the refresh token"}
	}
	if err := s.cache.Set(ctx, redeemedPrefix+codeKey, []byte(family), redeemedTTL); err != nil {
		slog.WarnContext(ctx, "oauth token: remember redeemed code", "err", err)
	}

	return refresh, nil
}

// refresh is the refresh_token grant: the token must be live, in date, and
// the client's; it is rotated in one statement, reuse revokes the family,
// and the scope is recomputed against what the map can still grant.
func (s *Server) refresh(ctx context.Context, form map[string][]string) (TokenResponse, *tokenError) {
	get := formGetter(form)
	raw := get("refresh_token")
	if raw == "" {
		return TokenResponse{}, &tokenError{code: ErrorInvalidRequest, description: "refresh_token is required"}
	}

	current, err := s.store.GetRefreshToken(ctx, hashToken(raw))
	if errors.Is(err, store.ErrNotFound) {
		return TokenResponse{}, &tokenError{code: ErrorInvalidGrant, description: "refresh_token is unknown"}
	}
	if err != nil {
		slog.ErrorContext(ctx, "oauth token: read refresh token", "err", err)

		return TokenResponse{}, &tokenError{code: ErrorTemporarilyUnavail, description: "could not read the refresh token"}
	}

	user, err := s.store.GetUser(ctx, current.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "oauth token: read user", "err", err)

		return TokenResponse{}, &tokenError{code: ErrorInvalidGrant, description: "refresh_token belongs to no one"}
	}
	principal := SubjectPrefix + user.Provider + ":" + user.Subject

	if terr := s.refreshable(ctx, &current, get("client_id")); terr != nil {
		terr.principal = principal

		return TokenResponse{}, terr
	}

	scopes, terr := s.refreshScopes(current.Scope, get("scope"))
	if terr != nil {
		terr.principal = principal

		return TokenResponse{}, terr
	}

	next, err := randomSecret()
	if err != nil {
		return TokenResponse{}, &tokenError{code: ErrorServer, description: errMint, principal: principal}
	}
	_, rotated, err := s.store.RotateRefreshToken(ctx, current.TokenHash, hashToken(next), joinScopes(scopes))
	if err != nil {
		slog.ErrorContext(ctx, "oauth token: rotate refresh token", "err", err)

		return TokenResponse{}, &tokenError{code: ErrorTemporarilyUnavail, description: "could not rotate the refresh token", principal: principal}
	}
	if !rotated {
		// Lost the race with another rotation of the same token: reuse.
		s.revokeFamily(ctx, current.FamilyID)

		return TokenResponse{}, &tokenError{code: ErrorInvalidGrant, description: errReused, principal: principal}
	}

	identity := auth.Identity{Provider: user.Provider, Subject: user.Subject, Email: user.Email, Login: user.Login}
	resp, terr := s.mint(&identity, current.ClientID, scopes, next)
	if terr != nil {
		terr.principal = principal

		return TokenResponse{}, terr
	}
	audit(ctx, GrantRefreshToken, current.ClientID, principal, "ok")

	return resp, nil
}

// refreshable says whether current may be rotated: not revoked, not already
// used (reuse revokes the family), not expired by disuse (which also ends
// the family), and the caller's client when it names one.
func (s *Server) refreshable(ctx context.Context, current *store.RefreshToken, clientID string) *tokenError {
	switch {
	case current.RevokedAt != nil:
		return &tokenError{code: ErrorInvalidGrant, description: "refresh_token is revoked"}
	case current.LastUsedAt != nil:
		s.revokeFamily(ctx, current.FamilyID)

		return &tokenError{code: ErrorInvalidGrant, description: errReused}
	case s.now().After(current.IssuedAt.Add(s.refreshTTL)):
		s.revokeFamily(ctx, current.FamilyID)

		return &tokenError{code: ErrorInvalidGrant, description: "refresh_token expired"}
	case clientID != "" && clientID != current.ClientID:
		return &tokenError{code: ErrorInvalidClient, description: "client_id is not the one the token was issued to"}
	}

	return nil
}

// refreshScopes is the rotated token's scope: what was granted and is
// still grantable, narrowed to the request's scope when it names one.
func (s *Server) refreshScopes(granted, requested string) ([]string, *tokenError) {
	scopes := s.currentScopes(strings.Fields(granted))
	asked := strings.Fields(requested)
	if len(asked) == 0 {
		return scopes, nil
	}
	for _, sc := range asked {
		if !slices.Contains(scopes, sc) {
			return nil, &tokenError{code: ErrorInvalidScope, description: "scope " + sc + " was not granted"}
		}
	}

	return asked, nil
}

// formGetter reads the first value of a form field.
func formGetter(form map[string][]string) func(string) string {
	return func(k string) string {
		if v := form[k]; len(v) > 0 {
			return v[0]
		}

		return ""
	}
}

// currentScopes keeps the granted scopes the server can still grant: an
// operator removing a scope from AUTHZ_SCOPE_MAP takes effect within one
// access lifetime. A person's groups live only in their site session, so a
// group they have lost keeps its scope until the refresh family ends.
func (s *Server) currentScopes(granted []string) []string {
	grantable := s.grantable()
	out := make([]string, 0, len(granted))
	for _, sc := range granted {
		if slices.Contains(grantable, sc) {
			out = append(out, sc)
		}
	}
	if len(out) == 0 {
		out = append(out, doczcfg.ScopePublicRead)
	}

	return out
}

// mint signs the access token for identity and builds the response.
func (s *Server) mint(identity *auth.Identity, clientID string, scopes []string, refresh string) (TokenResponse, *tokenError) {
	access, err := s.signer.Issue(&Claims{
		Subject:  SubjectPrefix + identity.Provider + ":" + identity.Subject,
		ClientID: clientID,
		Scopes:   scopes,
		Email:    identity.Email,
		Login:    identity.Login,
	})
	if err != nil {
		slog.Error("oauth token: sign access token", "err", err)

		return TokenResponse{}, &tokenError{code: ErrorServer, description: "could not sign the token"}
	}

	return TokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.signer.TTL().Seconds()),
		RefreshToken: refresh,
		Scope:        joinScopes(scopes),
	}, nil
}

func (s *Server) revokeFamily(ctx context.Context, familyID string) {
	if err := s.store.RevokeRefreshFamily(ctx, familyID); err != nil {
		slog.ErrorContext(ctx, "oauth token: revoke refresh family", "family", familyID, "err", err)
	}
}

// revoke is POST /oauth/revoke (RFC 7009): the refresh token's family is
// revoked, and an unknown token is 200 like a known one, so the endpoint
// confirms nothing.
func (s *Server) revoke(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, ErrorInvalidRequest, "the body must be application/x-www-form-urlencoded")

		return
	}
	raw := r.PostForm.Get("token")
	if raw == "" {
		writeOAuthError(w, ErrorInvalidRequest, "token is required")

		return
	}

	current, err := s.store.GetRefreshToken(r.Context(), hashToken(raw))
	switch {
	case errors.Is(err, store.ErrNotFound):
		// An access token or a stranger's guess: nothing to do.
	case err != nil:
		slog.ErrorContext(r.Context(), "oauth revoke: read refresh token", "err", err)
		writeOAuthError(w, ErrorTemporarilyUnavail, "could not read the token")

		return
	default:
		s.revokeFamily(r.Context(), current.FamilyID)
		audit(r.Context(), "revoke", current.ClientID, "user:"+itoa(current.UserID), "ok")
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}
