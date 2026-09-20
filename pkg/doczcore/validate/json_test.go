package validate

import (
	"encoding/json"
	"testing"
)

// TestSeverityJSON_RoundTrips pins the wire shape a consumer reads: a severity
// is its name, both ways. `docz validate --format json` and docz-api both serve
// findings, and a number here would make the enum's iota order part of the
// contract.
func TestSeverityJSON_RoundTrips(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Severity
		want string
	}{
		{name: "error", in: Error, want: `"error"`},
		{name: "warning", in: Warning, want: `"warning"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("Marshal(%v) = %v, want nil", tt.in, err)
			}

			if string(b) != tt.want {
				t.Errorf("Marshal(%v) = %s, want %s", tt.in, b, tt.want)
			}

			var back Severity
			if err := json.Unmarshal(b, &back); err != nil {
				t.Fatalf("Unmarshal(%s) = %v, want nil", b, err)
			}

			if back != tt.in {
				t.Errorf("round trip gave %v, want %v", back, tt.in)
			}
		})
	}
}

// TestSeverityJSON_RejectsUnknown pins that a name nobody writes is an error
// rather than a zero value. A consumer that read "eror" as "no severity" would
// drop a real finding out of its own report.
func TestSeverityJSON_RejectsUnknown(t *testing.T) {
	t.Parallel()

	for _, in := range []string{`"eror"`, `"Error"`, `""`, `1`} {
		var s Severity
		if err := json.Unmarshal([]byte(in), &s); err == nil {
			t.Errorf("Unmarshal(%s) = nil, want an error (got severity %v)", in, s)
		}
	}
}

// TestFindingJSON_Shape pins the finding's own keys, since they are what a
// consumer indexes on. Kind is omitted when empty because most findings do not
// concern a region, and a null there would read as "a region with no name".
func TestFindingJSON_Shape(t *testing.T) {
	t.Parallel()

	b, err := json.Marshal(Finding{
		Code:     "region.missing",
		Severity: Error,
		Line:     12,
		Detail:   "no objective region",
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	const want = `{"code":"region.missing","severity":"error","line":12,` +
		`"detail":"no objective region"}`

	if string(b) != want {
		t.Errorf("Marshal gave\n%s\nwant\n%s", b, want)
	}
}
