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

package config

import (
	"strings"
	"testing"
)

// TestValidateRepoRelativePath covers the rules shared by changelog.file
// and every api path. The changelog table in changelog_test.go exercises
// the same rules end to end through Validate; this one pins the helper
// directly so a rule can be read without a config around it, and so the
// allowDir branch — unreachable from the changelog block — is covered.
func TestValidateRepoRelativePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		allowDir bool
		wantErr  string // "" means the value must be accepted
	}{
		// Accepted shapes.
		{name: "plain file", value: "CHANGELOG.md"},
		{name: "nested file", value: "docs/examples/README.md"},
		{name: "dot prefixed name", value: ".changelog/CHANGELOG.md"},
		{name: "triple dot segment", value: ".../CHANGELOG.md"},
		{name: "colon later in path", value: "charts/a:b/CHANGELOG.md"},
		{name: "tilde later in path", value: "charts/~foo/CHANGELOG.md"},
		{name: "single letter dir", value: "c/x.md"},

		// Rejections that apply regardless of allowDir.
		{name: "empty", value: "", wantErr: "must not be empty"},
		{name: "empty as dir", value: "", allowDir: true, wantErr: "must not be empty"},
		{name: "nul byte", value: "CHANGE\x00LOG.md", wantErr: "control characters"},
		{name: "newline", value: "CHANGELOG.md\n", wantErr: "control characters"},
		{name: "backslash", value: `docs\CHANGELOG.md`, wantErr: "must use forward slashes"},
		{name: "absolute", value: "/etc/passwd", wantErr: "must be relative"},
		{name: "windows drive", value: "C:/Windows/x.md", wantErr: "must be relative"},
		{name: "windows drive lower", value: "c:/x.md", wantErr: "must be relative"},
		{name: "tilde home", value: "~/.ssh/id_rsa", wantErr: `must not start with "~"`},
		{name: "tilde user", value: "~root/x.md", wantErr: `must not start with "~"`},
		{name: "traversal", value: "../../etc/passwd", wantErr: "must not traverse"},
		{name: "interior traversal", value: "a/../b.md", wantErr: "must not traverse"},
		{name: "dot segment", value: "a/./b.md", wantErr: "must be a clean path"},
		{name: "empty segment", value: "a//b.md", wantErr: "must be a clean path"},
		{name: "leading dot slash", value: "./CHANGELOG.md", wantErr: "must be a clean path"},

		// allowDir is exactly one rule: a single trailing "/". Decision 1
		// — an api.exclude entry names a directory prefix, so "templates"
		// and "templates/" must mean the same thing.
		{name: "trailing slash as file", value: "templates/", wantErr: "not a directory"},
		{name: "trailing slash as dir", value: "templates/", allowDir: true},
		{name: "nested trailing slash as dir", value: "a/b/", allowDir: true},

		// ...and it must not weaken anything else. A trailing slash is
		// dropped before the empty-segment check, not the whole check.
		{
			name:     "double trailing slash as dir",
			value:    "templates//",
			allowDir: true,
			wantErr:  "must be a clean path",
		},
		{
			name:     "interior empty segment as dir",
			value:    "a//b/",
			allowDir: true,
			wantErr:  "must be a clean path",
		},
		{
			name:     "traversal as dir",
			value:    "../secrets/",
			allowDir: true,
			wantErr:  "must not traverse",
		},
		{
			name:     "absolute as dir",
			value:    "/etc/",
			allowDir: true,
			wantErr:  "must be relative",
		},
		{
			name:     "bare slash as dir",
			value:    "/",
			allowDir: true,
			wantErr:  "must be relative",
		},

		// Win32 strips trailing spaces from each path component, so a
		// segment ending in a space is traversal the ".." check cannot
		// see: ".. " resolves as "..". Found by security review; the old
		// inline validateChangelog rules had the same hole.
		{name: "space padded dotdot", value: ".. /x.md", wantErr: "ending in a space"},
		{name: "space padded dot", value: "a/. /b.md", wantErr: "ending in a space"},
		{
			name:     "space padded dotdot as dir",
			value:    ".. /",
			allowDir: true,
			wantErr:  "ending in a space",
		},
		{name: "space padded name", value: "docs /x.md", wantErr: "ending in a space"},
		{name: "trailing space on file", value: "x.md ", wantErr: "ending in a space"},
		// A space inside a component is an ordinary filename.
		{name: "interior space", value: "docs/release notes.md"},

		// Zero-width and bidi-override runes render as one path and
		// address another; C1 is unreachable in a real filename.
		{name: "rtl override", value: "\u202e../etc/passwd", wantErr: "control characters"},
		{name: "zero width space", value: "docs/\u200bx.md", wantErr: "control characters"},
		{name: "bom", value: "\ufeffx.md", wantErr: "control characters"},
		{name: "c1 control", value: "x\u0085.md", wantErr: "control characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var err error
			if tt.allowDir {
				err = validateRepoRelativeDir("", tt.value)
			} else {
				err = validateRepoRelativeFile("", tt.value)
			}
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("validateRepoRelativePath(%q, allowDir=%t) = %v, want nil",
					tt.value, tt.allowDir, err)
			case tt.wantErr != "" && err == nil:
				t.Errorf("validateRepoRelativePath(%q, allowDir=%t) = nil, want %q",
					tt.value, tt.allowDir, tt.wantErr)
			case tt.wantErr != "" && err != nil && !strings.Contains(err.Error(), tt.wantErr):
				t.Errorf("validateRepoRelativePath(%q, allowDir=%t) = %v, want it to contain %q",
					tt.value, tt.allowDir, err, tt.wantErr)
			}
		})
	}
}

// TestValidateRepoRelativePathFieldPrefix pins the two message shapes.
// A caller whose sentinel already names the key passes "" and gets a bare
// message so the rendered chain does not stutter; a caller whose sentinel
// covers several keys passes the field and gets it back in the message.
func TestValidateRepoRelativePathFieldPrefix(t *testing.T) {
	t.Parallel()

	bare := validateRepoRelativePath("", "../x.md", false)
	if bare == nil {
		t.Fatal("validateRepoRelativePath() = nil, want an error")
	}
	if got := bare.Error(); !strings.HasPrefix(got, `"../x.md"`) {
		t.Errorf("bare message = %q, want it to start with the offending value", got)
	}

	named := validateRepoRelativePath("api.exclude[1]", "../x.md", false)
	if named == nil {
		t.Fatal("validateRepoRelativePath() = nil, want an error")
	}
	if got := named.Error(); !strings.HasPrefix(got, "api.exclude[1] ") {
		t.Errorf("named message = %q, want it to start with the field", got)
	}
}
