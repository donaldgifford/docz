package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// vtestADRClean is an ADR carrying every region its schema requires, with
// consequences filled in, so a healthy document reports nothing at all. It is
// the baseline the other fixtures deviate from one thing at a time.
const vtestADRClean = `---
id: ADR-0001
title: "A Decision"
status: Proposed
author: Tester
created: 2026-01-01
---

# ADR-0001: A Decision

<!--toc:start-->
- [Summary](#summary)
- [Context](#context)
- [Decision](#decision)
- [Consequences](#consequences)
  - [Positive](#positive)
  - [Negative](#negative)
  - [Neutral](#neutral)
- [Alternatives Considered](#alternatives-considered)
- [References](#references)
<!--toc:end-->

<!--docz:summary:start-->
## Summary

We will do the thing.
<!--docz:summary:end-->

<!--docz:context:start-->
## Context

The thing was not being done.
<!--docz:context:end-->

<!--docz:decision:start-->
## Decision

Do the thing.
<!--docz:decision:end-->

<!--docz:consequences:start-->
## Consequences

<!--docz:positive:start-->
### Positive

- The thing is done.
<!--docz:positive:end-->

<!--docz:negative:start-->
### Negative

- Doing the thing takes time.
<!--docz:negative:end-->

<!--docz:neutral:start-->
### Neutral

- The thing exists.
<!--docz:neutral:end-->
<!--docz:consequences:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

- Not doing the thing.
<!--docz:alternatives:end-->

<!--docz:references:start-->
## References

- [RFC-0001](../rfc/0001-a-proposal.md)
<!--docz:references:end-->
`

// vtestADRReadme is a README index whose table is already what a fresh render
// would produce for the clean fixture, so index drift does not fire and the
// tests that care about drift can turn it on deliberately.
const vtestADRReadme = `# ADRs

<!-- BEGIN DOCZ AUTO-GENERATED -->
## All ADRs

| ID | Title | Status | Date | Author | Link |
|----|-------|--------|------|--------|------|
| ADR-0001 | A Decision | Proposed | 2026-01-01 | Tester | [0001-a-decision.md](0001-a-decision.md) |
<!-- END DOCZ AUTO-GENERATED -->
`

// vtestRunner builds a Runner over a temp repo with only `adr` enabled, so a
// run walks one type and a test can reason about every line of output.
//
// Only adr, because the other four built-ins would each contribute a template
// entry and an index-drift line for a directory the fixture never creates —
// noise that would make every assertion here a substring search.
func vtestRunner(t *testing.T, doc string) (*Runner, *bytes.Buffer, string) {
	t.Helper()

	root := t.TempDir()

	adrDir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(adrDir, config.DirMode); err != nil {
		t.Fatalf("mkdir %s: %v", adrDir, err)
	}

	vtestWrite(t, filepath.Join(adrDir, "0001-a-decision.md"), doc)
	vtestWrite(t, filepath.Join(adrDir, config.IndexFileName), vtestADRReadme)

	cfg := config.DefaultConfig()
	cfg.DocsDir = filepath.Join(root, "docs")

	for _, name := range []string{"rfc", "design", "impl", "investigation"} {
		tc := cfg.Types[name]
		tc.Enabled = false
		cfg.Types[name] = tc
	}

	var out bytes.Buffer

	return &Runner{
		Cfg:      cfg,
		Out:      &out,
		Err:      io.Discard,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:      time.Now,
		Git:      staticGit{},
		RepoRoot: root,
	}, &out, root
}

// vtestEmptyConsequences is the clean fixture with its three consequence
// regions present but empty.
//
// Emptied rather than removed: removing them is three region.missing errors,
// which would put the document over the default threshold and stop it
// separating "warnings only" from "errors". What is left is one warning, from
// pkg/adr, and nothing else.
func vtestEmptyConsequences() string {
	doc := vtestADRClean

	for _, pair := range [][2]string{
		{"positive", "Positive"},
		{"negative", "Negative"},
		{"neutral", "Neutral"},
	} {
		kind, heading := pair[0], pair[1]

		start := "<!--docz:" + kind + ":start-->"
		end := "<!--docz:" + kind + ":end-->"

		from := strings.Index(doc, start)
		to := strings.Index(doc, end) + len(end)

		doc = doc[:from] + start + "\n### " + heading + "\n" + end + doc[to:]
	}

	return doc
}

func vtestWrite(t *testing.T, path, body string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(body), config.FileMode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// vtestCodes returns the set of codes a text run printed, so a test asserts on
// what was found rather than on the exact wording after the code.
func vtestCodes(out string) map[string]bool {
	codes := make(map[string]bool)

	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			codes[fields[1]] = true
		}
	}

	return codes
}

// TestValidate_CleanDocumentExitsZero is the baseline: a document carrying
// every required region reports nothing and the command succeeds.
func TestValidate_CleanDocumentExitsZero(t *testing.T) {
	r, out, _ := vtestRunner(t, vtestADRClean)

	if err := r.validate(t.Context(), validateOpts{format: formatText}, nil); err != nil {
		t.Fatalf("validate = %v, want nil\noutput:\n%s", err, out.String())
	}

	if out.Len() != 0 {
		t.Errorf("a clean repository printed findings:\n%s", out.String())
	}
}

// TestValidate_TextFormatIsPathLineCodeDetail pins the line format, which is
// the contract a consumer greps.
func TestValidate_TextFormatIsPathLineCodeDetail(t *testing.T) {
	// Drop the whole references region: region.missing is a document-level
	// finding, so it exercises the line-0 case as well.
	doc := strings.Replace(vtestADRClean, `<!--docz:references:start-->
## References

- [RFC-0001](../rfc/0001-a-proposal.md)
<!--docz:references:end-->
`, "", 1)

	r, out, _ := vtestRunner(t, doc)

	err := r.validate(t.Context(), validateOpts{format: formatText}, nil)
	if err == nil {
		t.Fatalf("validate succeeded, want an error for a missing region\noutput:\n%s", out.String())
	}

	if got := exitCodeFor(err); got != 1 {
		t.Errorf("exit code = %d, want 1", got)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatal("no findings printed")
	}

	for _, line := range lines {
		fields := strings.SplitN(line, " ", 3)
		if len(fields) != 3 {
			t.Errorf("line %q is not `path:line code detail`", line)

			continue
		}

		path, _, ok := strings.Cut(fields[0], ":")
		if !ok {
			t.Errorf("line %q has no path:line prefix", line)

			continue
		}

		if !strings.HasPrefix(path, "docs/adr/") {
			t.Errorf("path %q is not repo-relative", path)
		}
	}

	if !vtestCodes(out.String())["region.missing"] {
		t.Errorf("want region.missing among the codes:\n%s", out.String())
	}
}

// TestValidate_StrictFailsOnWarningsAlone is what makes --strict the CI gate:
// the same report that exits 0 by default exits 1 with the flag.
func TestValidate_StrictFailsOnWarningsAlone(t *testing.T) {
	// An empty consequences region is a warning from the adr package and
	// nothing else, so this document separates the two thresholds.
	doc := vtestEmptyConsequences()

	r, out, _ := vtestRunner(t, doc)

	if err := r.validate(t.Context(), validateOpts{format: formatText}, nil); err != nil {
		t.Fatalf("validate without --strict = %v, want nil\noutput:\n%s", err, out.String())
	}

	warned := out.String()
	if warned == "" {
		t.Fatal("no warnings printed; the fixture no longer separates the thresholds")
	}

	r2, out2, _ := vtestRunner(t, doc)

	err := r2.validate(t.Context(), validateOpts{format: formatText, strict: true}, nil)
	if err == nil {
		t.Fatalf("validate --strict succeeded, want an error\noutput:\n%s", out2.String())
	}

	if got := exitCodeFor(err); got != 1 {
		t.Errorf("exit code = %d, want 1", got)
	}
}

// TestValidate_TypeTierRuns proves the five-arm switch is wired: this code
// comes from pkg/adr and from nowhere else, so its presence is the whole
// evidence that cmd composes the per-type tier onto the repository one.
func TestValidate_TypeTierRuns(t *testing.T) {
	doc := vtestEmptyConsequences()

	r, out, _ := vtestRunner(t, doc)

	if err := r.validate(t.Context(), validateOpts{format: formatText}, nil); err != nil {
		t.Fatalf("validate = %v, want nil\noutput:\n%s", err, out.String())
	}

	if !vtestCodes(out.String())["adr.consequences.empty"] {
		t.Errorf("want adr.consequences.empty from pkg/adr:\n%s", out.String())
	}
}

// TestValidate_JSONFormatIsTheReport pins the JSON shape: lower-case keys and
// a severity spelled out, which is what docz-api serves.
func TestValidate_JSONFormatIsTheReport(t *testing.T) {
	doc := strings.Replace(vtestADRClean, `<!--docz:references:start-->
## References

- [RFC-0001](../rfc/0001-a-proposal.md)
<!--docz:references:end-->
`, "", 1)

	r, out, _ := vtestRunner(t, doc)

	if err := r.validate(t.Context(), validateOpts{format: formatJSON}, nil); err == nil {
		t.Fatal("validate succeeded, want an error for a missing region")
	}

	var got struct {
		Docs []struct {
			Type     string `json:"type"`
			Path     string `json:"path"`
			Schema   string `json:"schema"`
			Findings []struct {
				Code     string `json:"code"`
				Severity string `json:"severity"`
				Line     int    `json:"line"`
				Detail   string `json:"detail"`
			} `json:"findings"`
		} `json:"docs"`
		Errors   int `json:"errors"`
		Warnings int `json:"warnings"`
	}

	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not the report as JSON: %v\n%s", err, out.String())
	}

	if len(got.Docs) != 1 {
		t.Fatalf("got %d docs, want 1", len(got.Docs))
	}

	doc0 := got.Docs[0]
	if doc0.Type != "adr" || doc0.Schema != "adr" {
		t.Errorf("got type %q schema %q, want adr adr", doc0.Type, doc0.Schema)
	}

	if doc0.Path != filepath.Join("docs", "adr", "0001-a-decision.md") {
		t.Errorf("path = %q, want the repo-relative document path", doc0.Path)
	}

	if got.Errors == 0 {
		t.Error("errors count is 0, want the missing region counted")
	}

	var sawError bool

	for _, f := range doc0.Findings {
		switch f.Severity {
		case "error":
			sawError = true
		case "warning":
		default:
			t.Errorf("severity %q is neither error nor warning", f.Severity)
		}
	}

	if !sawError {
		t.Errorf("no error-severity finding in the JSON:\n%s", out.String())
	}
}

// TestValidate_IndexDriftIsReported pins the drift line and that it fails only
// under --strict, since the fix is `docz update` rather than an edit.
func TestValidate_IndexDriftIsReported(t *testing.T) {
	r, out, root := vtestRunner(t, vtestADRClean)

	// Empty the generated table, which is exactly the drift `docz update`
	// would repair.
	vtestWrite(t, filepath.Join(root, "docs", "adr", config.IndexFileName),
		"# ADRs\n\n<!-- BEGIN DOCZ AUTO-GENERATED -->\n<!-- END DOCZ AUTO-GENERATED -->\n")

	if err := r.validate(t.Context(), validateOpts{format: formatText}, nil); err != nil {
		t.Fatalf("validate = %v, want nil: drift alone must not fail a default run", err)
	}

	if !vtestCodes(out.String())[codeIndexDrift] {
		t.Fatalf("want %s in the output:\n%s", codeIndexDrift, out.String())
	}

	r2, _, root2 := vtestRunner(t, vtestADRClean)
	vtestWrite(t, filepath.Join(root2, "docs", "adr", config.IndexFileName),
		"# ADRs\n\n<!-- BEGIN DOCZ AUTO-GENERATED -->\n<!-- END DOCZ AUTO-GENERATED -->\n")

	err := r2.validate(t.Context(), validateOpts{format: formatText, strict: true}, nil)
	if err == nil {
		t.Fatal("validate --strict succeeded, want drift to fail it")
	}

	if got := exitCodeFor(err); got != 1 {
		t.Errorf("exit code = %d, want 1", got)
	}
}

// TestValidate_UsageErrorsExitTwo covers both ways to use the command wrongly.
// Exit 2 is what separates "docz could not run" from "your documents are
// wrong", which is the distinction a CI job branches on.
func TestValidate_UsageErrorsExitTwo(t *testing.T) {
	tests := []struct {
		name string
		opts validateOpts
		args []string
		want string
	}{
		{
			name: "unknown format",
			opts: validateOpts{format: "yaml"},
			want: "invalid --format",
		},
		{
			name: "unknown type",
			opts: validateOpts{format: formatText},
			args: []string{"nonsense"},
			want: "unknown document type",
		},
		{
			name: "disabled type named explicitly",
			opts: validateOpts{format: formatText},
			args: []string{"rfc"},
			want: "is disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, out, _ := vtestRunner(t, vtestADRClean)

			err := r.validate(t.Context(), tt.opts, tt.args)
			if err == nil {
				t.Fatalf("validate succeeded, want a usage error\noutput:\n%s", out.String())
			}

			if got := exitCodeFor(err); got != 2 {
				t.Errorf("exit code = %d, want 2 (err %v)", got, err)
			}

			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not mention %q", err, tt.want)
			}
		})
	}
}
