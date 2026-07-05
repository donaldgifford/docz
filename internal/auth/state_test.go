package auth

import (
	"strings"
	"testing"
	"time"
)

func TestStateRoundTrip(t *testing.T) {
	t.Parallel()
	secret := []byte("state-signing-secret")

	state, err := EncodeState(secret, "github")
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}
	provider, err := VerifyState(secret, state)
	if err != nil {
		t.Fatalf("VerifyState: %v", err)
	}
	if provider != "github" {
		t.Errorf("provider = %q, want github", provider)
	}
}

func TestVerifyStateRejects(t *testing.T) {
	t.Parallel()
	secret := []byte("state-signing-secret")

	valid, err := EncodeState(secret, "okta")
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}
	expired, err := encodeState(secret, "okta", time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("encodeState expired: %v", err)
	}
	body, sig, _ := strings.Cut(valid, ".")
	tamperedBody := "A" + body[1:] + "." + sig
	badSigB64 := body + ".@@@not-base64"

	tests := []struct {
		name   string
		secret []byte
		state  string
	}{
		{"wrong secret", []byte("different-secret"), valid},
		{"tampered body", secret, tamperedBody},
		{"missing separator", secret, "noseparatorhere"},
		{"non-base64 signature", secret, badSigB64},
		{"expired", secret, expired},
		{"empty", secret, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, verr := VerifyState(tc.secret, tc.state); verr == nil {
				t.Errorf("VerifyState(%q) = nil error, want rejection", tc.name)
			}
		})
	}
}
