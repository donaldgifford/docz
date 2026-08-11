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
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

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
// "templates" and "templates/" must mean the same thing.
func validateRepoRelativePath(field, value string, allowDir bool) error {
	reject := func(format string, args ...any) error {
		msg := fmt.Sprintf(format, args...)
		if field == "" {
			return errors.New(msg)
		}
		return fmt.Errorf("%s %s", field, msg)
	}

	switch {
	case value == "":
		return reject("must not be empty")
	case strings.ContainsFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }):
		return reject("%q must not contain control characters", value)
	case strings.ContainsRune(value, '\\'):
		// Backslash is a path separator on Windows and a legal filename
		// character elsewhere, so a lone ".." check cannot judge it
		// portably. Repo-relative paths are slash-separated (that is how
		// git names them); requiring it keeps the traversal check below
		// meaningful on every host.
		return reject("%q must use forward slashes to separate directories", value)
	case filepath.IsAbs(value), strings.HasPrefix(value, "/"), hasVolumeName(value):
		return reject("%q must be relative to the repo root", value)
	case strings.HasPrefix(value, "~"):
		// Never expanded by docz, and a consumer that hands the path to a
		// shell would resolve it outside the repo entirely.
		return reject("%q must not start with %q", value, "~")
	case !allowDir && strings.HasSuffix(value, "/"):
		return reject("%q must be a file path, not a directory", value)
	}

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
		return reject("%q must not traverse outside the repo root", value)
	case slices.Contains(segments, "."), slices.Contains(segments, ""):
		return reject("%q must be a clean path: no %q or empty segments", value, ".")
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
