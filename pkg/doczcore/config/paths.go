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
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// normalizeRepoPath settles the spellings of a repo-relative path that
// mean the same file, so two configs that differ only cosmetically produce
// the same value: surrounding whitespace goes, and every leading "./" is
// stripped ("././docs/x.md" → "docs/x.md").
//
// It deliberately stops there. path.Clean would also resolve the ".." that
// validateRepoRelativePath must still be able to see and reject, turning a
// traversal attempt into a path that validates clean. Normalizing is not
// sanitizing: every value that passes through here is still judged.
//
// An empty result is left empty. Whether that is a mistake or a cue to
// backfill a default belongs to the caller, which is the only one that
// knows what the default is.
func normalizeRepoPath(value string) string {
	value = strings.TrimSpace(value)
	for strings.HasPrefix(value, "./") {
		value = value[len("./"):]
	}

	return value
}

// validateRepoRelativeFile checks a value that names a file, so a trailing
// "/" is a mistake. See validateRepoRelativePath for the rules.
func validateRepoRelativeFile(field, value string) error {
	return validateRepoRelativePath(field, value, false)
}

// validateRepoRelativeDir checks a value that names a directory prefix, so
// a single trailing "/" is accepted and means the same thing without it.
// See validateRepoRelativePath for the rules.
func validateRepoRelativeDir(field, value string) error {
	return validateRepoRelativePath(field, value, true)
}

// forbiddenPathRune reports whether r must never appear in a
// repo-relative path.
//
// Beyond C0 and DEL, this covers C1 (0x80–0x9f) and the Unicode format
// category, which is where the zero-width and bidirectional-override
// characters live. None of them can be typed into a real filename on
// purpose, and U+202E in particular reorders how the rest of a path
// renders — a path that displays as one thing in docz-site's UI while
// addressing another.
func forbiddenPathRune(r rune) bool {
	switch {
	case r < 0x20, r == 0x7f:
		return true
	case r >= 0x80 && r <= 0x9f:
		return true
	default:
		return unicode.Is(unicode.Cf, r)
	}
}

// validateRepoRelativePath applies docz's rules for a path that a consumer
// fetches out of a git tree: it must be relative to the repo root and
// already canonical. That rules out absolute paths, Windows volume names,
// ".." traversal, "." and empty segments, backslash separators, control
// characters, and a leading "~" that a shell would expand.
//
// Every rule is applied with docz's own semantics rather than the host
// OS's. This config is typically validated on a Linux runner for a path
// some other machine resolves, so filepath's platform-dependent view would
// make the verdict depend on who happened to run the check.
//
// The returned error is bare: callers wrap it with their own sentinel, so
// errors.Is stays precise per config block. field prefixes the message for
// a caller whose sentinel does not already identify the key — one
// ErrInvalidAPIPath covers three api fields, so "api.exclude[1]" has to
// come from here. Pass "" when the sentinel already names the key, as
// validateChangelog does, and the rendered chain reads without stuttering.
//
// allowDir keeps a trailing "/" legal. Pass true for a value that names a
// directory prefix rather than a file: api.exclude entries, where
// "templates" and "templates/" must mean the same thing. Most callers
// want validateRepoRelativeFile or validateRepoRelativeDir instead, which
// name the choice at the call site.
//
// The value is judged as the raw bytes git stores. It is deliberately not
// URL-decoded first, so "%2e%2e/x" is an ordinary directory name here and
// not traversal — a consumer that decodes a config-sourced path before
// resolving it has voided this check and must re-validate afterwards.
func validateRepoRelativePath(field, value string, allowDir bool) error {
	err := checkPathShape(value, allowDir)
	if err == nil || field == "" {
		return err
	}
	return fmt.Errorf("%s %w", field, err)
}

// checkPathShape holds the rules that judge the value as a whole. Split
// from the per-segment rules in checkPathSegments only to keep each
// function's branch count under the linter's ceiling; read the two as one
// ordered list, because which rule fires first is what the caller's
// message says.
func checkPathShape(value string, allowDir bool) error {
	switch {
	case value == "":
		return fmt.Errorf("must not be empty")
	case strings.ContainsFunc(value, forbiddenPathRune):
		return fmt.Errorf("%q must not contain control characters", value)
	case strings.ContainsRune(value, '\\'):
		// Backslash is a path separator on Windows and a legal filename
		// character elsewhere, so a lone ".." check cannot judge it
		// portably. Repo-relative paths are slash-separated (that is how
		// git names them); requiring it keeps the traversal check below
		// meaningful on every host.
		return fmt.Errorf("%q must use forward slashes to separate directories", value)
	case filepath.IsAbs(value), strings.HasPrefix(value, "/"), hasVolumeName(value):
		return fmt.Errorf("%q must be relative to the repo root", value)
	case strings.HasPrefix(value, "~"):
		// Never expanded by docz, and a consumer that hands the path to a
		// shell would resolve it outside the repo entirely.
		return fmt.Errorf("%q must not start with %q", value, "~")
	case !allowDir && strings.HasSuffix(value, "/"):
		return fmt.Errorf("%q must be a file path, not a directory", value)
	}

	return checkPathSegments(value, allowDir)
}

// checkPathSegments holds the rules that judge the value one "/"-separated
// component at a time. See checkPathShape for why this is a separate
// function.
func checkPathSegments(value string, allowDir bool) error {
	// Segments are split on "/" rather than handed to path.Clean so the
	// rejection can name what is wrong. Traversal is its own message
	// because it is the one a misconfigured repo actually hits.
	//
	// A directory prefix is allowed one trailing "/", which splits into a
	// final empty segment; drop it before the empty-segment check so
	// "templates/" is not read as "templates" plus a nameless child.
	segments := strings.Split(value, "/")
	if allowDir && len(segments) > 1 && segments[len(segments)-1] == "" {
		segments = segments[:len(segments)-1]
	}

	switch {
	case slices.Contains(segments, ".."):
		return fmt.Errorf("%q must not traverse outside the repo root", value)
	case slices.Contains(segments, "."), slices.Contains(segments, ""):
		return fmt.Errorf("%q must be a clean path: no %q or empty segments", value, ".")
	}

	// Win32 strips trailing spaces *and periods* from every path component,
	// so ".. " and "..." both resolve as ".." on a consumer that uses the
	// Windows API — traversal the segment checks above cannot see, because
	// the segment is not literally "..". The same trimming makes "docs."
	// name "docs", which is how a caller's own prefix rules get evaded.
	//
	// Checked last so it never changes which rule fires for a path the
	// other rules already reject.
	for _, seg := range segments {
		if strings.TrimRight(seg, " .") != seg {
			return fmt.Errorf(
				"%q must not have a path component ending in a space or period", value)
		}
	}

	return nil
}

// hasVolumeName reports whether p starts with a Windows drive letter such
// as "C:". filepath.VolumeName only recognizes one when the binary itself
// runs on Windows, and this config is routinely validated on a Linux
// runner for a path a consumer may resolve anywhere — the verdict must
// not depend on the validating host.
func hasVolumeName(p string) bool {
	if len(p) < 2 || p[1] != ':' {
		return false
	}
	c := p[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
