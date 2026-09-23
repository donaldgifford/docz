package config

import "log/slog"

// Secret is a string that must never appear in logs or formatted output.
//
// It redacts itself on every implicit stringification path — slog (via
// slog.LogValuer), fmt's %s/%v/%+v (via fmt.Stringer), and %#v (via
// fmt.GoStringer) — so an accidental slog.Any("cfg", cfg) or log of a struct
// that embeds a Secret prints "REDACTED", not the value. The underlying value
// is available only through an explicit Reveal, forcing callers to unwrap it
// consciously.
type Secret string

const redacted = "REDACTED"

// LogValue implements slog.LogValuer so slog output is always redacted.
func (Secret) LogValue() slog.Value {
	return slog.StringValue(redacted)
}

// String implements fmt.Stringer so %s/%v/%+v are redacted.
func (Secret) String() string {
	return redacted
}

// GoString implements fmt.GoStringer so %#v is redacted.
func (Secret) GoString() string {
	return redacted
}

// Reveal returns the underlying secret value. It is the only way to read the
// value, so every unwrap is explicit and greppable.
func (s Secret) Reveal() string {
	return string(s)
}
