package theme

import (
	"errors"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// namedColours are the CSS named colours a stylesheet is likeliest to
// reach for; transparent and currentColor are allowed.
const namedColours = "white|black|red|green|blue|gray|grey|silver|orange|" +
	"yellow|purple|pink|brown|navy|teal|maroon|olive|lime|aqua|fuchsia"

var (
	commentRE  = regexp.MustCompile(`(?s)/\*.*?\*/`)
	blockRE    = regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`)
	declRE     = regexp.MustCompile(`(--[a-z][a-z0-9-]*)\s*:\s*([^;]+);`)
	varRE      = regexp.MustCompile(`var\(\s*(--[a-z][a-z0-9-]*)`)
	jsTokenRE  = regexp.MustCompile(`["'](--[a-z][a-z0-9-]*)["']`)
	hexRE      = regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`)
	funcRE     = regexp.MustCompile(`\b(rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(`)
	cssValueRE = regexp.MustCompile(`:\s*([^;{}]+);`)
	namedRE    = regexp.MustCompile(`(?i)\b(` + namedColours + `)\b`)
	jsStringRE = regexp.MustCompile(`"[^"\n]*"|'[^'\n]*'`)
)

// blocks returns every innermost rule block in css, keyed by its
// selector, with the custom properties it declares.
func blocks(css string) map[string]map[string]string {
	css = commentRE.ReplaceAllString(css, "")
	out := make(map[string]map[string]string)
	for _, m := range blockRE.FindAllStringSubmatch(css, -1) {
		sel := strings.TrimSpace(m[1])
		if i := strings.LastIndexAny(sel, ";}"); i >= 0 {
			sel = strings.TrimSpace(sel[i+1:])
		}
		decls := make(map[string]string)
		for _, d := range declRE.FindAllStringSubmatch(m[2], -1) {
			decls[d[1]] = strings.TrimSpace(d[2])
		}
		out[sel] = decls
	}

	return out
}

// modes returns a theme's effective tokens per mode: light is the theme
// block, dark is the light block overridden by the data-mode="dark"
// block, and system-dark the same for the prefers-color-scheme block.
// classic has one block, which applies in every mode.
func modes(t *testing.T, name string) map[string]map[string]string {
	t.Helper()

	all := blocks(tokens[name])
	themeSel := `:root[data-theme="` + name + `"]`
	light, ok := all[themeSel]
	if !ok {
		t.Fatalf("tokens/%s.css: no %s block", name, themeSel)
	}

	out := map[string]map[string]string{"light": light}
	for mode, sel := range map[string]string{
		"dark":        themeSel + `[data-mode="dark"]`,
		"system-dark": themeSel + `:not([data-mode])`,
	} {
		over, ok := all[sel]
		if !ok {
			continue
		}
		merged := maps.Clone(light)
		maps.Copy(merged, over)
		out[mode] = merged
	}

	return out
}

// required is every token some theme defines: each theme must define all
// of them in every mode, so a component never reads an undefined value.
func required(t *testing.T) map[string]bool {
	t.Helper()

	req := make(map[string]bool)
	for _, name := range names {
		for _, decls := range modes(t, name) {
			for k := range decls {
				req[k] = true
			}
		}
	}

	return req
}

func TestTokenFilesMatchNames(t *testing.T) {
	t.Parallel()

	got := slices.Sorted(maps.Keys(tokens))
	want := slices.Sorted(slices.Values(names))
	if !slices.Equal(got, want) {
		t.Errorf("token files %v, want one per theme in Names() %v", got, want)
	}
}

func TestEveryThemeDefinesEveryToken(t *testing.T) {
	t.Parallel()

	req := required(t)
	for _, name := range names {
		for mode, decls := range modes(t, name) {
			for token := range req {
				if _, ok := decls[token]; !ok {
					t.Errorf("theme %s (%s) does not define %s", name, mode, token)
				}
			}
		}
	}
}

func TestPaperDarkBlocksAgree(t *testing.T) {
	t.Parallel()

	m := modes(t, "paper")
	dark, sys := m["dark"], m["system-dark"]
	if dark == nil || sys == nil {
		t.Fatal("paper needs a data-mode=dark block and a prefers-color-scheme block")
	}
	for k, v := range dark {
		if sys[k] != v {
			t.Errorf("paper %s: data-mode dark %q, system dark %q", k, v, sys[k])
		}
	}
}

func TestComponentsAndRuntimeReadOnlyDefinedTokens(t *testing.T) {
	t.Parallel()

	defined := required(t)
	for _, b := range []string{base, components} {
		for _, decls := range blocks(b) {
			for k := range decls {
				defined[k] = true
			}
		}
	}

	for file, re := range map[string]*regexp.Regexp{
		"components.css": varRE,
		"runtime.js":     jsTokenRE,
	} {
		src := components
		if file == "runtime.js" {
			src = runtime
		}
		for _, m := range re.FindAllStringSubmatch(commentRE.ReplaceAllString(src, ""), -1) {
			if !defined[m[1]] {
				t.Errorf("%s reads %s, which no theme defines", file, m[1])
			}
		}
	}
}

func TestComponentsAndRuntimeHaveNoLiteralColour(t *testing.T) {
	t.Parallel()

	css := commentRE.ReplaceAllString(components, "")
	for _, m := range cssValueRE.FindAllStringSubmatch(css, -1) {
		v := m[1]
		if hexRE.MatchString(v) || funcRE.MatchString(v) || namedRE.MatchString(v) {
			t.Errorf("components.css: literal colour in %q; use a token", strings.TrimSpace(v))
		}
	}

	js := commentRE.ReplaceAllString(runtime, "")
	for _, s := range jsStringRE.FindAllString(js, -1) {
		if hexRE.MatchString(s) || funcRE.MatchString(s) || namedRE.MatchString(s) {
			t.Errorf("runtime.js: literal colour in %s; read it with docz.palette", s)
		}
	}
}

func TestNames(t *testing.T) {
	t.Parallel()

	got := Names()
	if want := []string{"paper", "charcoal", "classic"}; !slices.Equal(got, want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}
	got[0] = "changed"
	if Names()[0] != "paper" {
		t.Error("Names() returned a slice that aliases the package's list")
	}
}

func TestCSS(t *testing.T) {
	t.Parallel()

	for _, name := range Names() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			css, err := CSS(name)
			if err != nil {
				t.Fatalf("CSS(%q) error: %v", name, err)
			}
			sel := strings.Index(css, `:root[data-theme="`+name+`"]`)
			comp := strings.Index(css, components)
			if !strings.HasPrefix(css, base) || sel < 0 || comp < sel {
				t.Errorf("CSS(%q) is not base, then the theme's tokens, then the components", name)
			}
			for _, other := range Names() {
				if other != name && strings.Contains(css, `:root[data-theme="`+other+`"]`) {
					t.Errorf("CSS(%q) carries the %s tokens", name, other)
				}
			}
		})
	}
}

func TestCSSUnknownTheme(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", "Paper", "dark"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			css, err := CSS(name)
			if !errors.Is(err, ErrUnknownTheme) {
				t.Fatalf("CSS(%q) error = %v, want ErrUnknownTheme", name, err)
			}
			if css != "" {
				t.Errorf("CSS(%q) returned a stylesheet with its error", name)
			}
			msg := err.Error()
			for _, want := range []string{`"` + name + `"`, "paper", "charcoal", "classic"} {
				if !strings.Contains(msg, want) {
					t.Errorf("CSS(%q) error %q does not mention %s", name, msg, want)
				}
			}
		})
	}
}

func TestRuntime(t *testing.T) {
	t.Parallel()

	if !strings.Contains(Runtime(), "window.docz =") {
		t.Error("Runtime() does not define window.docz")
	}
}
