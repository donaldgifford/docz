package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/v2/pkg/adr"
	"github.com/donaldgifford/docz/v2/pkg/design"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
	"github.com/donaldgifford/docz/v2/pkg/impl"
	"github.com/donaldgifford/docz/v2/pkg/investigation"
	"github.com/donaldgifford/docz/v2/pkg/rfc"
)

// codeIndexDrift is the code `docz validate` prints for a drifted README
// index.
//
// The library reports index drift as an IndexDrift rather than a Finding,
// because the table is generated and the fix is `docz update` rather than an
// edit (DESIGN-0015 §4). The CLI still prints it in the finding line format:
// a user skimming output wants one shape, and a code is what they would
// filter on either way.
const codeIndexDrift = "index.drift"

// errValidateFailed marks a run that found something at or above the
// caller's threshold. Its own text is never shown — the summary message
// wrapped around it is.
var errValidateFailed = errors.New("validation failed")

var (
	validateStrict bool
	validateFormat string
	validateFix    bool
)

// validateOpts is the per-invocation flag state, packed so the handler
// never reads a global.
type validateOpts struct {
	strict bool
	format string
	fix    bool
}

// validateFixOutput is what `--format json --fix` emits: the pass and the
// report it left behind, in one object.
//
// A second bare report would be two JSON documents on one stream, which is
// not a thing a consumer can decode. The `fixed` key is how a consumer knows
// which shape it has.
type validateFixOutput struct {
	Fixed  *repo.InsertRegionsReport `json:"fixed"`
	Report *repo.ValidateReport      `json:"report"`
}

var validateCmd = &cobra.Command{
	Use:   "validate [type]",
	Short: "Check documents against their schema and report drift",
	Long: `Validate documents against the regions their schema requires, the content
rules of each region kind, their frontmatter, and the drift only a
regeneration can see: a stale table of contents, and a README index table
that is not what 'docz update' would write.

With no type argument every enabled type is checked.

Findings print one per line as 'path:line code detail'. The code is the
stable part: the wording after it may change, the code will not.

With --fix, documents whose regions were read by inference get those regions
marked, and markers docz read leniently are rewritten in the canonical
spelling. Nothing else is changed: a section the document does not have is
not invented, so what --fix cannot repair is reported afterwards for you to
fix by hand. A second --fix over the same repository writes nothing.

Exit codes:
  0  nothing found, or warnings only without --strict
  1  errors were found, or anything was found with --strict
  2  the command was used wrongly (unknown type, bad --format)

` + config.TypesHelp(),
	Args: cobra.MaximumNArgs(1),
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().BoolVar(&validateStrict, "strict", false,
		"fail on warnings and index drift as well as errors")
	validateCmd.Flags().StringVar(&validateFormat, "format", formatText,
		"output format: text or json")
	validateCmd.Flags().BoolVar(&validateFix, "fix", false,
		"mark the regions inference finds, then report what is left")
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	opts := validateOpts{strict: validateStrict, format: validateFormat, fix: validateFix}

	return getRunner().validate(cmdContext(cmd), opts, args)
}

// validate runs the repository tier, composes the per-type tier onto it,
// prints the result, and returns the exit code as an error.
//
// The composition is the point of the whole three-tier shape (ADR-0002
// Decision 4): repo may not import a type package, so nothing dispatches on
// a document's type except this function, in a switch a reader can see all
// of at once.
func (r *Runner) validate(ctx context.Context, opts validateOpts, args []string) error {
	format, err := resolveStatusFormat(opts.format)
	if err != nil {
		return err
	}

	rp := r.repoOrOpen()

	report, err := r.validatePass(ctx, rp, args, opts)
	if err != nil {
		return validateExitError(err)
	}

	if !opts.fix {
		if err := r.printValidateReport(format, &report); err != nil {
			return err
		}

		return validateOutcome(&report, opts.strict)
	}

	return r.validateFix(ctx, rp, format, args, opts, &report)
}

// validatePass is one full validation: the repository tier plus the per-type
// tier composed onto it.
//
// Extracted because `--fix` runs it twice — once to find what to mark, once to
// report what is left — and a fix that reported against a different set of
// checks than it decided from would be reporting about a different repository.
func (r *Runner) validatePass(
	ctx context.Context, rp *repo.Repo, args []string, opts validateOpts,
) (repo.ValidateReport, error) {
	report, err := rp.Validate(ctx, args, repo.ValidateOptions{Strict: opts.strict})
	if err != nil {
		return report, err
	}

	r.applyTypeTier(rp, &report)

	return report, nil
}

// validateFix marks the regions inference found, then reports what is left.
//
// The first report is never printed. Its job is to decide whether there is
// anything to mark; printing findings that the next paragraph of output has
// already fixed would be the most confusing thing this command could do.
// What the user sees is the pass, then the state of the repository after it,
// and the exit code is the second report's.
func (r *Runner) validateFix(
	ctx context.Context,
	rp *repo.Repo,
	format string,
	args []string,
	opts validateOpts,
	first *repo.ValidateReport,
) error {
	var fixed repo.InsertRegionsReport

	if types := fixableTypes(first); len(types) > 0 {
		var err error

		fixed, err = rp.InsertRegions(ctx, types, repo.InsertRegionsOptions{})
		if err != nil {
			// The report is not printed on this path. A pass that aborted
			// mid-write leaves a repository whose state is the error's to
			// describe, and re-validating it would bury that behind a
			// hundred findings.
			return validateExitError(err)
		}
	}

	second, err := r.validatePass(ctx, rp, args, opts)
	if err != nil {
		return validateExitError(err)
	}

	if format == formatJSON {
		enc := json.NewEncoder(r.Out)
		enc.SetIndent("", "  ")

		if err := enc.Encode(validateFixOutput{Fixed: &fixed, Report: &second}); err != nil {
			return err
		}

		return validateOutcome(&second, opts.strict)
	}

	if err := r.printFixReport(&fixed); err != nil {
		return err
	}

	if err := r.printValidateText(&second); err != nil {
		return err
	}

	return validateOutcome(&second, opts.strict)
}

// fixableTypes names the types holding at least one document the migration
// pass would touch.
//
// Types rather than documents, because that is the granularity
// repo.InsertRegions works at. Nothing is lost by widening: the pass is
// idempotent and never re-marks a document that already carries a region, so
// the extra documents in a named type are read and left alone. Narrowing to
// the exact document list would mean a second API that could disagree with
// the first about what "already marked" means.
//
// The two codes are the two things the pass can repair. region.inferred says
// a document was read by inference and would be marked; marker.spelling says
// its markers are docz's but not canonically spelled, which the pass rewrites
// in place. Anything else in the report is for an author to fix by hand, and
// running the pass would not help.
func fixableTypes(report *repo.ValidateReport) []string {
	seen := make(map[string]bool)

	var types []string

	for i := range report.Docs {
		doc := &report.Docs[i]

		for _, f := range doc.Findings {
			if f.Code != validate.CodeRegionInferred && f.Code != validate.CodeMarkerSpelling {
				continue
			}

			if !seen[doc.Type] {
				seen[doc.Type] = true
				types = append(types, doc.Type)
			}

			break
		}
	}

	return types
}

// printFixReport writes one line per document the pass changed.
//
// Nothing is printed for a document it left alone. A repository of a hundred
// already-marked documents should say nothing about them, so that the handful
// of lines it does print are the whole of what changed.
func (r *Runner) printFixReport(report *repo.InsertRegionsReport) error {
	for _, res := range report.Changed {
		parts := make([]string, 0, 2)

		if len(res.Inserted) > 0 {
			parts = append(parts, "marked "+strings.Join(res.Inserted, ", "))
		}

		if res.Fixed > 0 {
			parts = append(parts, "canonicalized "+plural(res.Fixed, "marker"))
		}

		if _, err := fmt.Fprintf(r.Out, "%s: %s\n",
			res.Path, strings.Join(parts, "; ")); err != nil {
			return err
		}
	}

	return nil
}

// applyTypeTier appends each document's type-specific findings to the
// generic ones.
//
// Documents only. A Templates entry is a rendered template validated against
// its schema, checked with placeholder metadata so a sound template reports
// nothing (repo.checkTemplate); running a type's content rules over comment
// prose and placeholders would report a finding on every run of every
// healthy repository, which is the fastest way to teach somebody to ignore
// output.
//
// A document whose bytes cannot be re-read keeps its generic findings and
// gains a note rather than failing the run: the generic tier already read it
// once, so a failure here is a race with something else writing the tree, and
// losing the rest of the report to it would be the wrong trade.
func (r *Runner) applyTypeTier(rp *repo.Repo, report *repo.ValidateReport) {
	for i := range report.Docs {
		doc := &report.Docs[i]

		check := typeValidator(doc.Schema, doc.Type)
		if check == nil {
			continue
		}

		content, err := os.ReadFile(rp.Path(doc.Path))
		if err != nil {
			r.Logger.Debug("type-specific checks skipped",
				"path", doc.Path, "err", err)

			continue
		}

		doc.Findings = append(doc.Findings, check(content)...)

		sortFindings(doc.Findings)
	}

	retally(report)
}

// typeValidator returns the per-type checker for a document, or nil when no
// type package claims it.
//
// The switch is on the schema the document validated against, falling back to
// its type name, because the schema is what the document said it was: a
// custom `spike` type whose documents declare `schema: investigation` gets
// the investigation rules, which is the whole reason the grammar is over
// region kinds rather than type names (ADR-0002 R7).
//
// Nil for anything else, and that is the normal case for a custom type. A
// registry keyed by name would let a type package be reached without being
// imported, which is exactly the dispatch ADR-0002 Decision 4 rules out; an
// explicit switch means the set of built-in types is visible in one place and
// the linker can see every package the CLI depends on.
func typeValidator(schema, typeName string) func([]byte) []validate.Finding {
	name := schema
	if name == "" {
		name = typeName
	}

	switch name {
	case "rfc":
		return rfc.Validate
	case "adr":
		return adr.Validate
	case "design":
		return design.Validate
	case "impl":
		return impl.Validate
	case "investigation":
		return investigation.Validate
	default:
		return nil
	}
}

// sortFindings puts a document's findings in the order a reader walks it: by
// line, then by code.
//
// The same rule validate applies within its own tier, applied again here
// because two tiers cannot merge-sort without agreeing on a tie-break neither
// of them owns — so the caller that holds both does it (repo.DocFindings).
func sortFindings(findings []validate.Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}

		return findings[i].Code < findings[j].Code
	})
}

// retally recounts the report's severity totals after the per-type tier has
// appended to it.
//
// repo.Validate counts what it found, which is no longer the whole report by
// the time this runs. Recounting rather than incrementing as findings arrive,
// for the same reason repo does: a total that disagrees with the findings
// beside it is worse than no total.
func retally(report *repo.ValidateReport) {
	report.Errors, report.Warnings = 0, 0

	for _, group := range [][]repo.DocFindings{report.Docs, report.Templates} {
		for i := range group {
			for _, f := range group[i].Findings {
				switch f.Severity {
				case validate.Error:
					report.Errors++
				case validate.Warning:
					report.Warnings++
				}
			}
		}
	}
}

// printValidateReport writes the report in the requested format.
func (r *Runner) printValidateReport(format string, report *repo.ValidateReport) error {
	if format == formatJSON {
		enc := json.NewEncoder(r.Out)
		enc.SetIndent("", "  ")

		return enc.Encode(report)
	}

	return r.printValidateText(report)
}

// printValidateText writes one line per finding, then one per drifted index.
//
// Templates come before documents, matching the order the checks ran in and
// the order a reader needs: a template that lost a region reports the same
// region missing from every document made from it, so seeing the template
// first is seeing the cause first.
func (r *Runner) printValidateText(report *repo.ValidateReport) error {
	for _, group := range [][]repo.DocFindings{report.Templates, report.Docs} {
		for i := range group {
			if err := r.printDocFindings(&group[i]); err != nil {
				return err
			}
		}
	}

	for _, drift := range report.Index {
		if _, err := fmt.Fprintf(r.Out, "%s:0 %s %s\n", drift.Path, codeIndexDrift,
			"index table is out of date; run docz update to regenerate it"); err != nil {
			return err
		}
	}

	return nil
}

// printDocFindings writes one entry's findings.
//
// An entry with an empty path is docz's own embedded template, which has no
// file a person could open; it is named by its type instead, so a finding
// about it is still attributable.
func (r *Runner) printDocFindings(entry *repo.DocFindings) error {
	path := entry.Path
	if path == "" {
		path = "<embedded " + entry.Type + " template>"
	}

	for _, f := range entry.Findings {
		if _, err := fmt.Fprintf(r.Out, "%s:%d %s %s\n",
			path, f.Line, f.Code, f.Detail); err != nil {
			return err
		}
	}

	return nil
}

// validateOutcome maps a report onto the process exit code.
//
// Errors always fail. Warnings and index drift fail only under --strict,
// which is what makes the flag the CI gate (docz issue #97): a stale ToC and
// a drifted index are both `docz update`'s work rather than an author's, so a
// developer running validate by hand should see them without being stopped by
// them.
func validateOutcome(report *repo.ValidateReport, strict bool) error {
	drift := len(report.Index)

	if report.Errors == 0 && (!strict || report.Warnings+drift == 0) {
		return nil
	}

	return exitErrorf(errValidateFailed, "%s", validateSummary(report))
}

// validateSummary is the one-line count the failure message carries.
func validateSummary(report *repo.ValidateReport) string {
	parts := []string{
		plural(report.Errors, "error"),
		plural(report.Warnings, "warning"),
	}

	if drift := len(report.Index); drift > 0 {
		parts = append(parts, plural(drift, "drifted index"))
	}

	out := parts[0]
	for _, p := range parts[1:] {
		out += ", " + p
	}

	return out
}

// plural renders a count with its noun, adding an s for anything but one.
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}

	return fmt.Sprintf("%d %ss", n, noun)
}

// validateExitError maps a repo.Validate failure onto an exit code.
//
// A token the user got wrong is a usage error and exits 2, the same code
// `docz status set` uses for one. Everything else — an unreadable directory,
// a cancelled run — exits 1.
func validateExitError(err error) error {
	var unknown *repo.UnknownTypeError
	if errors.As(err, &unknown) {
		return exitErrorf(errExitCode2, "%v", unknown)
	}

	var disabled *repo.TypeDisabledError
	if errors.As(err, &disabled) {
		return exitErrorf(errExitCode2, "%v", disabled)
	}

	return err
}
