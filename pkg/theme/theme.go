package theme

import (
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrUnknownTheme is returned by [CSS] for a name [Names] does not list.
var ErrUnknownTheme = errors.New("unknown theme")

var (
	//go:embed base.css
	base string

	//go:embed components.css
	components string

	//go:embed runtime.js
	runtime string

	//go:embed tokens/paper.css
	paperTokens string

	//go:embed tokens/classic.css
	classicTokens string
)

// names is the one list of themes, in the order Names reports them; the
// first is the default.
var names = []string{"paper", "classic"}

// tokens maps each theme to its token file. theme_test.go holds its keys
// equal to names.
var tokens = map[string]string{
	"paper":   paperTokens,
	"classic": classicTokens,
}

// sheets is each theme's full stylesheet, built once.
var sheets = buildSheets()

func buildSheets() map[string]string {
	out := make(map[string]string, len(tokens))
	for name, t := range tokens {
		out[name] = strings.Join([]string{base, t, components}, "\n")
	}

	return out
}

// Names lists the themes in a stable order: "paper", then "classic". The
// first is the default. The slice is the caller's to keep.
func Names() []string {
	return slices.Clone(names)
}

// CSS returns the named theme's stylesheet, ready to inline in a <style>
// element: the page font tokens, the theme's token values, and the
// shared components. A page selects the theme with data-theme on its
// root element; the stylesheet styles nothing without it. An unknown
// name, including "", is an error wrapping [ErrUnknownTheme].
func CSS(name string) (string, error) {
	sheet, ok := sheets[name]
	if !ok {
		return "", fmt.Errorf("%w %q (valid: %s)", ErrUnknownTheme, name, strings.Join(names, ", "))
	}

	return sheet, nil
}

// Runtime returns the page runtime, ready to inline in a <script>
// element: the mode toggle, tabs, copy buttons, the IMPL task filter,
// and in-page links. It defines one global, docz.
func Runtime() string {
	return runtime
}
