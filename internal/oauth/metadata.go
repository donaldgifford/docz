package oauth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"

	"github.com/go-jose/go-jose/v4"

	doczcfg "github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// The three documents docz-api publishes as an issuer (DESIGN-0023 §5).
const (
	ProtectedResourcePath   = "/.well-known/oauth-protected-resource"
	AuthorizationServerPath = "/.well-known/oauth-authorization-server"
	JWKSPath                = "/.well-known/jwks.json"
	AuthorizePath           = "/oauth/authorize"
	TokenPath               = "/oauth/token" //nolint:gosec // a route path, not a credential
	RevokePath              = "/oauth/revoke"
)

// ProtectedResource is RFC 9728's document for the MCP endpoint.
type ProtectedResource struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	ScopesSupported        []string `json:"scopes_supported"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
	ResourceName           string   `json:"resource_name"`
}

// AuthorizationServer is RFC 8414's document for docz-api's own issuer.
type AuthorizationServer struct {
	Issuer                                     string   `json:"issuer"`
	AuthorizationEndpoint                      string   `json:"authorization_endpoint"`
	TokenEndpoint                              string   `json:"token_endpoint"`
	RevocationEndpoint                         string   `json:"revocation_endpoint"`
	JWKSURI                                    string   `json:"jwks_uri"`
	ScopesSupported                            []string `json:"scopes_supported"`
	ResponseTypesSupported                     []string `json:"response_types_supported"`
	GrantTypesSupported                        []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported              []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported          []string `json:"token_endpoint_auth_methods_supported"`
	RevocationEndpointAuthMethodsSupported     []string `json:"revocation_endpoint_auth_methods_supported"`
	AuthorizationResponseIssParameterSupported bool     `json:"authorization_response_iss_parameter_supported"`
	ClientIDMetadataDocumentSupported          bool     `json:"client_id_metadata_document_supported"`
}

// Metadata builds the documents for an issuer whose MCP endpoint is
// audience and whose scope map can grant scopes beyond public:read.
type Metadata struct {
	issuer   string
	audience string
	scopes   []string
	jwks     jose.JSONWebKeySet
}

// NewMetadata returns the documents for issuer. scopeMapScopes is every
// scope AUTHZ_SCOPE_MAP can grant; public:read is always first.
func NewMetadata(issuer, audience string, scopeMapScopes []string, jwks jose.JSONWebKeySet) *Metadata {
	return &Metadata{issuer: issuer, audience: audience, scopes: scopesSupported(scopeMapScopes), jwks: jwks}
}

// scopesSupported is public:read followed by the map's scopes, sorted and
// de-duplicated.
func scopesSupported(extra []string) []string {
	rest := slices.Clone(extra)
	rest = slices.DeleteFunc(rest, func(s string) bool { return s == doczcfg.ScopePublicRead })
	slices.Sort(rest)

	return append([]string{doczcfg.ScopePublicRead}, slices.Compact(rest)...)
}

// ScopesSupported is what the metadata advertises, and so the widest scope
// an authorization may ask for.
func (m *Metadata) ScopesSupported() []string { return slices.Clone(m.scopes) }

// ProtectedResource is the RFC 9728 document.
func (m *Metadata) ProtectedResource() ProtectedResource {
	return ProtectedResource{
		Resource:               m.audience,
		AuthorizationServers:   []string{m.issuer},
		ScopesSupported:        m.ScopesSupported(),
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "docz-api",
	}
}

// AuthorizationServer is the RFC 8414 document.
func (m *Metadata) AuthorizationServer() AuthorizationServer {
	return AuthorizationServer{
		Issuer:                                     m.issuer,
		AuthorizationEndpoint:                      m.issuer + AuthorizePath,
		TokenEndpoint:                              m.issuer + TokenPath,
		RevocationEndpoint:                         m.issuer + RevokePath,
		JWKSURI:                                    m.issuer + JWKSPath,
		ScopesSupported:                            m.ScopesSupported(),
		ResponseTypesSupported:                     []string{responseTypeCode},
		GrantTypesSupported:                        []string{GrantAuthorizationCode, GrantRefreshToken},
		CodeChallengeMethodsSupported:              []string{"S256"},
		TokenEndpointAuthMethodsSupported:          []string{"none"},
		RevocationEndpointAuthMethodsSupported:     []string{"none"},
		AuthorizationResponseIssParameterSupported: true,
		ClientIDMetadataDocumentSupported:          true,
	}
}

// JWKS is the public key set.
func (m *Metadata) JWKS() jose.JSONWebKeySet { return m.jwks }

// Mount serves the three documents on r, outside every gate; each is
// public by definition.
func (m *Metadata) Mount(r interface {
	Get(pattern string, h http.HandlerFunc)
},
) {
	r.Get(ProtectedResourcePath, func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, m.ProtectedResource()) })
	r.Get(AuthorizationServerPath, func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, m.AuthorizationServer()) })
	r.Get(JWKSPath, func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, m.jwks) })
}

// writeJSON writes v with the status; a document is never cached by a
// shared cache for long, since the key set rotates with the deployment.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"error":"server_error"}`, http.StatusInternalServerError)

		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if _, werr := w.Write(body); werr != nil {
		slog.Debug("oauth response write failed", "err", werr)
	}
}
