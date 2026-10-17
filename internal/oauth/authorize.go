package oauth

import (
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"

	doczcfg "github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// Parameter names and values the flow spells more than once.
const (
	responseTypeCode = "code"
	paramCode        = "code"
	paramError       = "error"
	paramState       = "state"
)

// OAuth error codes the authorization and token endpoints return.
const (
	ErrorInvalidRequest      = "invalid_request"
	ErrorInvalidClient       = "invalid_client"
	ErrorInvalidGrant        = "invalid_grant"
	ErrorInvalidScope        = "invalid_scope"
	ErrorInvalidTarget       = "invalid_target"
	ErrorUnsupportedGrant    = "unsupported_grant_type"
	ErrorUnsupportedResponse = "unsupported_response_type"
	ErrorAccessDenied        = "access_denied"
	ErrorServer              = "server_error"
	ErrorTemporarilyUnavail  = "temporarily_unavailable"
)

// authorize is GET /oauth/authorize (DESIGN-0023 §5): validate in order,
// store the request pending, send the person to the consent page. A
// failure before the redirect_uri is trusted is an error page; after, an
// error redirect with state and iss.
func (s *Server) authorize(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	q := r.URL.Query()

	// Pre-trust: nothing below may redirect until the client and its
	// redirect_uri are known good.
	if rt := q.Get("response_type"); rt != responseTypeCode {
		errorPage(w, ErrorUnsupportedResponse, "response_type must be code")

		return
	}

	clientID := q.Get("client_id")
	meta, err := s.clients.Resolve(r.Context(), clientID)
	if err != nil {
		slog.DebugContext(r.Context(), "oauth authorize: client refused", "client_id", clientID, "err", err)
		errorPage(w, ErrorInvalidClient, "the client_id is not a usable Client ID Metadata Document URL")

		return
	}

	redirectURI := q.Get("redirect_uri")
	if !slices.Contains(meta.RedirectURIs, redirectURI) {
		errorPage(w, ErrorInvalidRequest, "redirect_uri is not one the client registered")

		return
	}

	// Post-trust: the client gets the error.
	fail := func(code, description string) {
		params := url.Values{paramError: {code}, "error_description": {description}}
		if st := q.Get("state"); st != "" {
			params.Set("state", st)
		}
		http.Redirect(w, r, s.redirectWith(redirectURI, params), http.StatusFound) //nolint:gosec // redirectURI is one the client registered
	}

	challenge := q.Get("code_challenge")
	if q.Get("code_challenge_method") != "S256" || !validPKCE(challenge) {
		fail(ErrorInvalidRequest, "code_challenge with code_challenge_method=S256 is required")

		return
	}
	if q.Get("resource") != s.audience {
		fail(ErrorInvalidTarget, "resource must be "+s.audience)

		return
	}
	scopes, err := s.requestedScopes(q.Get("scope"))
	if err != nil {
		fail(ErrorInvalidScope, err.Error())

		return
	}
	state := q.Get("state")
	if state == "" {
		fail(ErrorInvalidRequest, "state is required")

		return
	}

	id, err := randomSecret()
	if err != nil {
		fail(ErrorServer, "could not store the request")

		return
	}
	p := pending{
		ClientID: meta.ClientID, ClientName: meta.Name(), ClientURI: meta.ClientURI,
		RedirectURI: redirectURI, CodeChallenge: challenge, Scopes: scopes, State: state,
		ExpiresAt: s.now().Add(pendingTTL),
	}
	if err := s.put(r.Context(), pendingPrefix+id, p, pendingTTL); err != nil {
		slog.ErrorContext(r.Context(), "oauth authorize: store pending request", "err", err)
		fail(ErrorTemporarilyUnavail, "could not store the request")

		return
	}

	http.Redirect(w, r, s.siteURL+"/oauth/consent?authz="+url.QueryEscape(id), http.StatusFound)
}

// requestedScopes parses the scope parameter: empty means public:read, and
// every token must be one the metadata advertises.
func (s *Server) requestedScopes(raw string) ([]string, error) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return []string{doczcfg.ScopePublicRead}, nil
	}

	grantable := s.grantable()
	out := make([]string, 0, len(fields))
	for _, sc := range fields {
		if !slices.Contains(grantable, sc) {
			return nil, fmt.Errorf("scope %q is not one this server grants", sc)
		}
		if !slices.Contains(out, sc) {
			out = append(out, sc)
		}
	}

	return out, nil
}

// errorPage is the pre-trust failure: a plain page naming the error,
// because there is no redirect_uri yet that may be trusted with it.
func errorPage(w http.ResponseWriter, code, description string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	if _, werr := fmt.Fprintf(w, "<!doctype html><html><head><title>docz: authorization failed</title></head>"+
		"<body><h1>Authorization failed</h1><p><code>%s</code>: %s</p></body></html>\n",
		html.EscapeString(code), html.EscapeString(description)); werr != nil {
		slog.Debug("oauth error page write failed", "err", werr)
	}
}

// oauthError is the RFC 6749 §5.2 error body.
type oauthError struct {
	Error       string `json:"error"`
	Description string `json:"error_description,omitempty"`
}

// writeOAuthError writes an RFC 6749 error. invalid_client is 401 as the
// RFC says; everything else is 400.
func writeOAuthError(w http.ResponseWriter, code, description string) {
	status := http.StatusBadRequest
	if code == ErrorInvalidClient {
		status = http.StatusUnauthorized
	}
	if code == ErrorServer {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, oauthError{Error: code, Description: description})
}

// errorFor maps a refusal to its OAuth code.
func errorFor(err error) string {
	if errors.Is(err, ErrInvalidClient) {
		return ErrorInvalidClient
	}

	return ErrorInvalidGrant
}
