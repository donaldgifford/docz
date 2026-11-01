// Copyright 2026 Donald Gifford
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config_test

import (
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

func TestDefaultConfig_HTMLThemeIsPaper(t *testing.T) {
	t.Parallel()

	if got := config.DefaultConfig().HTML.Theme; got != config.DefaultTheme {
		t.Errorf("DefaultConfig().HTML.Theme = %q, want %q", got, config.DefaultTheme)
	}
	if config.DefaultTheme != "paper" {
		t.Errorf("DefaultTheme = %q, want %q", config.DefaultTheme, "paper")
	}
}

func TestParseBytes_HTMLTheme(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		yaml     string
		want     string
		warnings int
	}{
		{name: "absent block is paper", yaml: "docs_dir: docs\n", want: "paper"},
		{name: "absent key is paper", yaml: "html: {}\n", want: "paper"},
		{name: "empty value is paper", yaml: "html:\n  theme: \"\"\n", want: "paper"},
		{name: "a known theme is kept", yaml: "html:\n  theme: classic\n", want: "classic"},
		{name: "trimmed and folded", yaml: "html:\n  theme: \" Classic \"\n", want: "classic"},
		{
			name:     "an unknown theme is paper with a warning",
			yaml:     "html:\n  theme: neon\n",
			want:     "paper",
			warnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := config.ParseBytes([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("ParseBytes: %v", err)
			}
			if cfg.HTML.Theme != tt.want {
				t.Errorf("HTML.Theme = %q, want %q", cfg.HTML.Theme, tt.want)
			}

			warnings, err := cfg.Validate()
			if err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
			if got := countThemeWarnings(warnings); got != tt.warnings {
				t.Errorf("Validate() theme warnings = %d, want %d: %q", got, tt.warnings, warnings)
			}
		})
	}
}

// TestValidate_HTMLThemeWarnsEveryRun pins that the warning survives a
// second Validate: repo.Open validates and drops warnings, and cmd/
// validates again to print them.
func TestValidate_HTMLThemeWarnsEveryRun(t *testing.T) {
	t.Parallel()

	cfg, err := config.ParseBytes([]byte("html:\n  theme: neon\n"))
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	for run := range 2 {
		warnings, err := cfg.Validate()
		if err != nil {
			t.Fatalf("Validate() run %d error = %v", run, err)
		}
		if got := countThemeWarnings(warnings); got != 1 {
			t.Errorf("Validate() run %d theme warnings = %d, want 1", run, got)
		}
	}
}

// TestValidate_HTMLThemeOnHandBuiltConfig covers a Config no Load
// normalised: an unknown name still warns and falls back.
func TestValidate_HTMLThemeOnHandBuiltConfig(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.HTML.Theme = "neon"

	warnings, err := cfg.Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got := countThemeWarnings(warnings); got != 1 {
		t.Errorf("Validate() theme warnings = %d, want 1: %q", got, warnings)
	}
	if cfg.HTML.Theme != config.DefaultTheme {
		t.Errorf("HTML.Theme after Validate = %q, want %q", cfg.HTML.Theme, config.DefaultTheme)
	}
}

func countThemeWarnings(warnings []string) int {
	n := 0
	for _, w := range warnings {
		if strings.HasPrefix(w, "html.theme ") {
			n++
		}
	}
	return n
}
