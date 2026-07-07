package auth

import "slices"

// Registry holds the enabled providers keyed by Name. It is built once at
// startup from config and read concurrently by the login/callback handlers.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry indexes providers by their Name.
func NewRegistry(providers []Provider) *Registry {
	m := make(map[string]Provider, len(providers))
	for _, p := range providers {
		m[p.Name()] = p
	}
	return &Registry{providers: m}
}

// Get returns the named provider and whether it is enabled.
func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// Names returns the enabled provider names in sorted order.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
