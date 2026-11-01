package archive_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/theme"
)

// TestChartThemeEnumMatchesPkgTheme pins charts/docz's site.config.theme
// enum to pkg/theme.Names() plus "" (no DOCZ_THEME), so a theme added to
// pkg/theme cannot be refused at install, and a removed one cannot be
// installed (IMPL-0030 Phase 3).
func TestChartThemeEnumMatchesPkgTheme(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "charts", "docz", "values.schema.json"))
	if err != nil {
		t.Fatal(err)
	}

	var schema struct {
		Properties struct {
			Site struct {
				Properties struct {
					Config struct {
						Properties struct {
							Theme struct {
								Enum []string `json:"enum"`
							} `json:"theme"`
						} `json:"properties"`
					} `json:"config"`
				} `json:"properties"`
			} `json:"site"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}

	got := slices.Sorted(slices.Values(schema.Properties.Site.Properties.Config.Properties.Theme.Enum))
	want := slices.Sorted(slices.Values(append([]string{""}, theme.Names()...)))
	if !slices.Equal(got, want) {
		t.Errorf("site.config.theme enum = %q, want %q (pkg/theme.Names() and \"\")", got, want)
	}
}
