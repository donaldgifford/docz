package oauth

import (
	"crypto"
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// Signer mints docz-api's own access tokens: ES256 over the configured key,
// with the key's RFC 7638 thumbprint as kid so the JWKS and the token agree
// without a name the operator has to manage.
type Signer struct {
	issuer   string
	audience string
	key      *ecdsa.PrivateKey
	kid      string
	ttl      time.Duration
	now      func() time.Time
}

// NewSigner builds a Signer for issuer, whose tokens carry audience and live
// ttl from issue.
func NewSigner(issuer, audience string, key *ecdsa.PrivateKey, ttl time.Duration) (*Signer, error) {
	kid, err := KeyID(&key.PublicKey)
	if err != nil {
		return nil, err
	}

	return &Signer{issuer: issuer, audience: audience, key: key, kid: kid, ttl: ttl, now: time.Now}, nil
}

// KeyID returns the RFC 7638 thumbprint of pub, base64url without padding,
// the kid docz-api uses for its own key.
func KeyID(pub *ecdsa.PublicKey) (string, error) {
	sum, err := (&jose.JSONWebKey{Key: pub}).Thumbprint(crypto.SHA256)
	if err != nil {
		return "", fmt.Errorf("key thumbprint: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(sum), nil
}

// JWKS is the public key set the issuer publishes at /.well-known/jwks.json.
func (s *Signer) JWKS() jose.JSONWebKeySet {
	return jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key: &s.key.PublicKey, KeyID: s.kid, Algorithm: string(jose.ES256), Use: "sig",
	}}}
}

// Issue signs a token for c: iss, sub, aud, exp, iat, jti, and the scope,
// client_id, email, and login claims. c.Expiry and c.Issuer are set by the
// signer; c.ID is generated when empty.
func (s *Signer) Issue(c *Claims) (string, error) {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: s.key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", s.kid),
	)
	if err != nil {
		return "", fmt.Errorf("new signer: %w", err)
	}

	now := s.now()
	id := c.ID
	if id == "" {
		id, err = randomID()
		if err != nil {
			return "", err
		}
	}

	std := jwt.Claims{
		Issuer:    s.issuer,
		Subject:   c.Subject,
		Audience:  jwt.Audience{s.audience},
		Expiry:    jwt.NewNumericDate(now.Add(s.ttl)),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ID:        id,
	}
	custom := claimSet{
		ClientID: c.ClientID,
		Email:    c.Email,
		Login:    c.Login,
		Scope:    joinScopes(c.Scopes),
	}

	raw, err := jwt.Signed(signer).Claims(std).Claims(custom).Serialize()
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return raw, nil
}

// TTL is the lifetime of a token the signer issues.
func (s *Signer) TTL() time.Duration { return s.ttl }
