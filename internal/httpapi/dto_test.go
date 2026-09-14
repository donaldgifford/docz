package httpapi

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// TestNullTimestampPinsUTC covers the zone pin. pgx scans a timestamptz into
// the process's own location unless ScanLocation is set, which docz-api never
// sets — so an unpinned render carried the host's offset and the same instant
// served differently from a container and a laptop. The rendered stamp must
// be the UTC spelling whatever location the value arrives in, and must stay
// byte-identical to what the search layer emits for the same instant.
func TestNullTimestampPinsUTC(t *testing.T) {
	instant := time.Date(2025, time.June, 22, 18, 4, 11, 0, time.UTC)
	const want = "2025-06-22T18:04:11Z"

	zones := []struct {
		name string
		loc  *time.Location
	}{
		{"already UTC", time.UTC},
		{"west of UTC", time.FixedZone("EDT", -4*60*60)},
		{"east of UTC", time.FixedZone("CEST", 2*60*60)},
		{"a half-hour offset", time.FixedZone("IST", 5*60*60+30*60)},
	}
	for _, z := range zones {
		t.Run(z.name, func(t *testing.T) {
			ts := pgtype.Timestamptz{Time: instant.In(z.loc), Valid: true}
			if got := nullTimestamp(ts); got != want {
				t.Errorf("nullTimestamp in %s = %q, want %q", z.name, got, want)
			}
		})
	}

	// NULL stays the wire's not-applicable convention, never a zero instant.
	if got := nullTimestamp(pgtype.Timestamptz{}); got != "" {
		t.Errorf("nullTimestamp(NULL) = %q, want empty", got)
	}
}
