package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// stateTTL bounds how long an in-flight login may take from /auth/login to
// /auth/callback before its state is rejected.
const stateTTL = 5 * time.Minute

// statePayload is the CSRF state carried through the OAuth round-trip. It also
// records the provider, so /auth/callback — which providers redirect to without
// echoing the provider — knows which provider to Exchange with.
type statePayload struct {
	Provider  string    `json:"p"`
	Nonce     string    `json:"n"`
	ExpiresAt time.Time `json:"e"`
}

// EncodeState builds a signed, expiring state value for provider. The value is
// `<base64url(payload)>.<base64url(hmac)>`, signed with secret (the session
// secret) so a forged or tampered state fails VerifyState.
func EncodeState(secret []byte, provider string) (string, error) {
	return encodeState(secret, provider, time.Now().Add(stateTTL))
}

// encodeState is EncodeState with an explicit expiry, for testability.
func encodeState(secret []byte, provider string, expiresAt time.Time) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate state nonce: %w", err)
	}
	data, err := json.Marshal(statePayload{
		Provider:  provider,
		Nonce:     base64.RawURLEncoding.EncodeToString(nonce),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return "", fmt.Errorf("marshal state: %w", err)
	}
	body := base64.RawURLEncoding.EncodeToString(data)
	sig := base64.RawURLEncoding.EncodeToString(sign(secret, body))
	return body + "." + sig, nil
}

// VerifyState checks a state value's signature and expiry and returns the
// provider it carries. It fails closed on any tampering, malformed input, or
// expiry, so a caller can treat any error as "reject this callback".
func VerifyState(secret []byte, raw string) (string, error) {
	body, sig, ok := strings.Cut(raw, ".")
	if !ok {
		return "", errors.New("malformed state")
	}
	gotSig, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(sign(secret, body), gotSig) {
		return "", errors.New("invalid state signature")
	}
	data, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return "", errors.New("invalid state encoding")
	}
	var payload statePayload
	if uerr := json.Unmarshal(data, &payload); uerr != nil {
		return "", fmt.Errorf("decode state: %w", uerr)
	}
	if time.Now().After(payload.ExpiresAt) {
		return "", errors.New("state expired")
	}
	return payload.Provider, nil
}

// sign returns the HMAC-SHA256 of msg under secret. hash.Hash.Write never
// errors, so its result is ignored (see the errcheck exclusion).
func sign(secret []byte, msg string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(msg))
	return mac.Sum(nil)
}
