package export

import (
	"context"
	"errors"
	"net/http"

	"github.com/donaldgifford/docz/v2/pkg/export/confluence"
)

// classify maps an export's error to the run's status and whether retrying
// could help (DESIGN-0021 §5, Failures). A nil error succeeded. A
// configuration or credential problem needs an operator, and a page that
// failed to render renders the same bytes next time, so neither is retried;
// a page that failed on a 429, a 5xx, or the network is. A cancelled context
// is a shutdown: failed, and re-queued by the worker without counting a
// retry.
func classify(err error) (status Status, skipRetry bool) {
	if err == nil {
		return StatusSucceeded, false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return StatusFailed, false
	}

	var ae *confluence.AuthError
	if errors.Is(err, confluence.ErrConfig) || errors.As(err, &ae) {
		return StatusFailed, true
	}

	var pe *confluence.PageError
	if errors.As(err, &pe) {
		return StatusPartial, !transient(pe.Err)
	}

	return StatusFailed, !transient(err)
}

// transient reports an error a retry may clear: a 429, a 5xx, or a request
// that got no response at all. A malformed render, a taken title, or any
// other 4xx is the same next time.
func transient(err error) bool {
	var (
		me *confluence.MalformedError
		te *confluence.TitleError
		re *confluence.RequestError
	)

	switch {
	case errors.As(err, &me), errors.As(err, &te):
		return false
	case errors.As(err, &re):
		return re.Status == 0 || re.Status == http.StatusTooManyRequests || re.Status >= http.StatusInternalServerError
	}

	return true
}
