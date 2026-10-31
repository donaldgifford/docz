package html

import "context"

// Hooks are callbacks Export calls as it works, carried in the context so
// a package that prints nothing can still be narrated (the shape of
// pkg/doczcore/repo's and pkg/export/confluence's). A nil field is not
// called.
type Hooks struct {
	// PageDone is called after each page, written, unchanged, or failed.
	PageDone func(PageResult)
}

type hooksKey struct{}

// WithHooks returns ctx carrying h. A nil h clears inherited hooks.
func WithHooks(ctx context.Context, h *Hooks) context.Context {
	return context.WithValue(ctx, hooksKey{}, h)
}

// HooksFrom returns the hooks ctx carries, never nil.
func HooksFrom(ctx context.Context) *Hooks {
	if h, ok := ctx.Value(hooksKey{}).(*Hooks); ok && h != nil {
		return h
	}

	return &Hooks{}
}

func firePageDone(ctx context.Context, r *PageResult) {
	if f := HooksFrom(ctx).PageDone; f != nil {
		f(*r)
	}
}
