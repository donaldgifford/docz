package cmd

import (
	"context"
	"testing"
	"time"
)

func TestStaticGit_UserName(t *testing.T) {
	tests := []struct {
		name string
		git  staticGit
		want string
	}{
		{"named", staticGit{Name: "Alice"}, "Alice"},
		{"empty", staticGit{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.git.UserName(context.Background()); got != tt.want {
				t.Errorf("UserName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRealGit_UserName_Smoke verifies realGit does not panic and
// returns either the configured git user.name or "". The exact value
// depends on the test environment, so the assertion is permissive.
func TestRealGit_UserName_Smoke(_ *testing.T) {
	_ = realGit{}.UserName(context.Background())
}

// TestRealGit_UserName_CtxCancel pins the Phase 6 contract: realGit's
// lookup honors context cancellation. An already-cancelled context
// must cause UserName to return "" promptly without panicking — this
// is the property that lets a Ctrl+C during `docz create` interrupt a
// hung `git config user.name` shellout (DESIGN-0004 §H).
//
// The test bounds wall-clock with t.Deadline-style timeout so a future
// regression where the ctx is dropped surfaces as a timeout rather than
// a hung test.
func TestRealGit_UserName_CtxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before invoking

	done := make(chan string, 1)
	go func() { done <- realGit{}.UserName(ctx) }()

	select {
	case got := <-done:
		if got != "" {
			t.Errorf("expected empty string on cancelled ctx, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("realGit.UserName did not return within 2s for a cancelled ctx — the ctx is not being honored")
	}
}
