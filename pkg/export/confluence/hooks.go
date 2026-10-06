package confluence

import "context"

// Hooks narrate an export. Export prints nothing and holds no logger
// (DESIGN-0020 §1); a caller that wants to say what is happening installs
// hooks in the context with WithHooks, in the same shape as repo.Hooks.
// Every field is optional.
type Hooks struct {
	// PageDone fires after each page is reconciled, with its result.
	PageDone func(PageResult)
	// Request fires after each HTTP response the client receives, with the
	// request method, the URL path, and the status code. It never carries a
	// header, so it never carries a credential.
	Request func(method, path string, status int)
}

// hooksKey is the context key for Hooks.
type hooksKey struct{}

// WithHooks returns a copy of ctx carrying h. A nil h clears hooks the
// caller inherited.
func WithHooks(ctx context.Context, h *Hooks) context.Context {
	return context.WithValue(ctx, hooksKey{}, h)
}

// HooksFrom returns the hooks carried by ctx, never nil.
func HooksFrom(ctx context.Context) *Hooks {
	if h, ok := ctx.Value(hooksKey{}).(*Hooks); ok && h != nil {
		return h
	}

	return &Hooks{}
}

func fireRequest(ctx context.Context, method, path string, status int) {
	if fn := HooksFrom(ctx).Request; fn != nil {
		fn(method, path, status)
	}
}

func firePageDone(ctx context.Context, r *PageResult) {
	if fn := HooksFrom(ctx).PageDone; fn != nil {
		fn(*r)
	}
}
