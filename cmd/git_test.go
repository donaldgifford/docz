package cmd

import (
	"context"
	"testing"
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
// IMPL-0009 Phase 6 adds a stricter test that exercises context
// cancellation.
func TestRealGit_UserName_Smoke(_ *testing.T) {
	_ = realGit{}.UserName(context.Background())
}
