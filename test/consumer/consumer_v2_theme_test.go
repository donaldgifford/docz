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

package consumer_test

import (
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// TestExternalConsumerReadsHTMLTheme proves the html: block (DESIGN-0025
// §3, IMPL-0030 Phase 1) is reachable from outside the module: the default
// is paper, and a manifest's theme parses and folds through ParseBytes.
func TestExternalConsumerReadsHTMLTheme(t *testing.T) {
	t.Parallel()

	if got := config.DefaultConfig().HTML.Theme; got != config.DefaultTheme {
		t.Errorf("DefaultConfig().HTML.Theme = %q, want %q", got, config.DefaultTheme)
	}

	cfg, err := config.ParseBytes([]byte("html:\n  theme: Classic\n"))
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if cfg.HTML.Theme != "classic" {
		t.Errorf("HTML.Theme = %q, want %q", cfg.HTML.Theme, "classic")
	}
}
