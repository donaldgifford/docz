package consumer

// DESIGN-0025's HTML layer exercised from outside the module: pkg/theme,
// the one design an exported page inlines and docz-site imports. A
// consumer rendering its own docz HTML reaches the same stylesheet and
// runtime the docz binary embeds.

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/theme"
)

func TestExternalConsumerReadsTheThemes(t *testing.T) {
	names := theme.Names()
	if !slices.Equal(names, []string{"paper", "classic"}) {
		t.Fatalf("theme.Names() = %v, want [paper classic]", names)
	}

	for _, name := range names {
		css, err := theme.CSS(name)
		if err != nil {
			t.Fatalf("theme.CSS(%q): %v", name, err)
		}
		if !strings.Contains(css, `:root[data-theme="`+name+`"]`) {
			t.Errorf("theme.CSS(%q) does not carry its own tokens", name)
		}
	}

	if _, err := theme.CSS("nope"); !errors.Is(err, theme.ErrUnknownTheme) {
		t.Errorf("theme.CSS(nope) error = %v, want ErrUnknownTheme", err)
	}
	if theme.Runtime() == "" {
		t.Error("theme.Runtime() is empty")
	}
}
