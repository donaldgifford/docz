// Package oauth is docz-api's token layer (DESIGN-0023 §5, §6): the verifier
// that accepts a JWT minted by docz-api's own key or by any trusted issuer,
// and the authorization server that mints them. Verification is local: an
// issuer's keys are fetched once and cached, and a token is never looked up.
//
// Nothing here reads a session or decides authorization; the one authn
// middleware (internal/session) turns Claims into a Session, and
// internal/authorize turns scopes into repositories.
package oauth
