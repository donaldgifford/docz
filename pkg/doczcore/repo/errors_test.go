package repo

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// One test per error shape (IMPL-0018 Phase 3 task 2). What matters for each
// is not its English but that the facts a caller branches on survive: the
// fields are populated, and the two errors that wrap something still answer
// errors.Is through the wrap.

func TestNotFoundError(t *testing.T) {
	t.Parallel()

	err := &NotFoundError{Type: "impl", ID: "IMPL-0099"}

	for _, want := range []string{"impl", "IMPL-0099"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not mention %q", err.Error(), want)
		}
	}

	var target *NotFoundError
	if !errors.As(error(err), &target) {
		t.Error("errors.As did not match *NotFoundError")
	}
}

func TestTypeDisabledError(t *testing.T) {
	t.Parallel()

	err := &TypeDisabledError{Type: "plan"}

	if !strings.Contains(err.Error(), "plan") {
		t.Errorf("message %q does not name the type", err.Error())
	}

	// Distinct from UnknownTypeError on purpose: the fix is a config edit,
	// not a different argument, and a caller that conflated them would tell
	// the user to check their spelling.
	var unknown *UnknownTypeError
	if errors.As(error(err), &unknown) {
		t.Error("a disabled type matched *UnknownTypeError")
	}
}

func TestExistsError(t *testing.T) {
	t.Parallel()

	err := &ExistsError{Path: "docs/templates/rfc.md"}

	if !strings.Contains(err.Error(), "docs/templates/rfc.md") {
		t.Errorf("message %q does not name the path", err.Error())
	}
}

func TestInvalidStatusError(t *testing.T) {
	t.Parallel()

	err := &InvalidStatusError{
		Type:    "adr",
		Status:  "Nonsense",
		Allowed: []string{"Proposed", "Accepted"},
	}

	msg := err.Error()

	// Allowed is in the message and on the struct, so a caller can either
	// print the library's wording or compose its own two-line form.
	for _, want := range []string{"Nonsense", "adr", "Proposed", "Accepted"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not mention %q", msg, want)
		}
	}

	var target *InvalidStatusError
	if !errors.As(error(err), &target) {
		t.Fatal("errors.As did not match *InvalidStatusError")
	}

	if len(target.Allowed) != 2 {
		t.Errorf("Allowed = %v, want both statuses", target.Allowed)
	}
}

func TestUnknownTypeError(t *testing.T) {
	t.Parallel()

	err := &UnknownTypeError{Token: "rfcs", Valid: []string{"rfc", "adr"}}

	// The frozen v1 sentinel still answers, which is what lets a consumer
	// written against config.ValidateType keep working unchanged.
	if !errors.Is(error(err), config.ErrUnknownType) {
		t.Error("errors.Is(err, config.ErrUnknownType) = false, want true")
	}

	var target *UnknownTypeError
	if !errors.As(error(err), &target) {
		t.Fatal("errors.As did not match *UnknownTypeError")
	}

	if target.Token != "rfcs" {
		t.Errorf("Token = %q, want the token verbatim", target.Token)
	}

	if !strings.Contains(err.Error(), "rfc, adr") {
		t.Errorf("message %q does not list the valid types", err.Error())
	}
}

func TestWriteError(t *testing.T) {
	t.Parallel()

	err := &WriteError{Path: "docs/rfc/0001-x.md", Err: fs.ErrPermission}

	if !strings.Contains(err.Error(), "docs/rfc/0001-x.md") {
		t.Errorf("message %q does not name the path", err.Error())
	}

	// The path comes off the struct and the cause stays reachable, so a
	// caller does not have to choose between the two.
	if !errors.Is(error(err), fs.ErrPermission) {
		t.Error("errors.Is(err, fs.ErrPermission) = false, want true")
	}

	var target *WriteError
	if !errors.As(error(err), &target) {
		t.Fatal("errors.As did not match *WriteError")
	}

	if target.Path != "docs/rfc/0001-x.md" {
		t.Errorf("Path = %q, want the path", target.Path)
	}
}

// TestResolveType covers the shared front half: which of the two type errors
// a token gets, and that resolution is as lenient here as on the command line.
func TestResolveType(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	tc := cfg.Types["adr"]
	tc.Enabled = false
	cfg.Types["adr"] = tc

	r := &Repo{Root: t.TempDir(), Cfg: &cfg}

	tests := []struct {
		name    string
		token   string
		want    string
		wantErr string // "", "unknown", or "disabled"
	}{
		{name: "canonical name", token: "rfc", want: "rfc"},
		{name: "upper case", token: "RFC", want: "rfc"},
		{name: "registry alias", token: "inv", want: "investigation"},
		{name: "id prefix", token: "IMPL", want: "impl"},
		{name: "unresolvable", token: "nope", wantErr: "unknown"},
		{name: "disabled", token: "adr", wantErr: "disabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := r.resolveType(tt.token)

			switch tt.wantErr {
			case "unknown":
				var target *UnknownTypeError
				if !errors.As(err, &target) {
					t.Fatalf("err = %v, want *UnknownTypeError", err)
				}

				return
			case "disabled":
				var target *TypeDisabledError
				if !errors.As(err, &target) {
					t.Fatalf("err = %v, want *TypeDisabledError", err)
				}

				if target.Type != "adr" {
					t.Errorf("Type = %q, want the canonical name", target.Type)
				}

				return
			}

			if err != nil {
				t.Fatalf("resolveType(%q) = %v, want nil", tt.token, err)
			}

			if got != tt.want {
				t.Errorf("resolveType(%q) = %q, want %q", tt.token, got, tt.want)
			}
		})
	}
}
