package confluence

import (
	"errors"
	"fmt"
)

// ErrConfig is wrapped by every ConfigError, so a caller can test for any
// configuration failure with errors.Is.
var ErrConfig = errors.New("confluence: configuration")

// ConfigError reports an export that cannot start: the sync.confluence block
// absent, disabled, or naming something that is not there.
type ConfigError struct {
	Reason string
}

func (e *ConfigError) Error() string { return "confluence: " + e.Reason }

func (*ConfigError) Unwrap() error { return ErrConfig }

// AuthError reports a 401 or 403 from Confluence: the credentials are wrong,
// or a scoped token lacks a scope. Its fix is configuration.
type AuthError struct {
	Op     string
	Status int
	// Body is the start of the response, which for a scoped token names the
	// scope it lacks.
	Body string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("confluence: %s: not authorized (HTTP %d): %s", e.Op, e.Status, e.Body)
}

// ConflictError reports a version conflict on update: the page moved on
// since it was read. Have is the version the page is at when known, Want the
// version the write expected to replace.
type ConflictError struct {
	Title string
	Have  int
	Want  int
}

func (e *ConflictError) Error() string {
	if e.Have == 0 {
		return fmt.Sprintf("confluence: %q changed since it was read (expected v%d)", e.Title, e.Want)
	}

	return fmt.Sprintf("confluence: %q is at v%d, expected v%d", e.Title, e.Have, e.Want)
}

// RequestError reports any other failed request.
type RequestError struct {
	Op     string
	Status int
	// Body is the start of the response, at most maxErrorBody bytes.
	Body string
	// Err is the transport error, when there was no response.
	Err error
}

func (e *RequestError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("confluence: %s: %v", e.Op, e.Err)
	}

	return fmt.Sprintf("confluence: %s: HTTP %d: %s", e.Op, e.Status, e.Body)
}

func (e *RequestError) Unwrap() error { return e.Err }

// maxErrorBody caps the response body an error carries.
const maxErrorBody = 512
