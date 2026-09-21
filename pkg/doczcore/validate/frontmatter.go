package validate

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
)

// codeIDPrefix is the one code covering every way an id can be wrong in its
// prefix half: absent, unparseable, or not the type's. They are one fix for
// an author, so they are one code.
const codeIDPrefix = "frontmatter.id-prefix"

var (
	// idPattern splits a frontmatter id into its prefix and number.
	idPattern = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9]*)-(\d+)$`)

	// schemaNamePattern is the schema name grammar (DESIGN-0015 §3).
	schemaNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

// checkFrontmatter reports the frontmatter findings, plus schema.name.
//
// A document with no frontmatter gets one finding and no others from this
// group: every later check would report the same absence again, and a list
// of six findings that are all one problem is worse than one.
func checkFrontmatter(content []byte, opts *Options) []Finding {
	fm, err := document.ParseFrontmatter(content)
	if err != nil {
		severity := Error

		detail := err.Error()
		if errors.Is(err, document.ErrNoFrontmatter) {
			detail = "document has no YAML frontmatter block"
		}

		return []Finding{{
			Code:     "frontmatter.missing",
			Severity: severity,
			Line:     1,
			Detail:   detail,
		}}
	}

	out := checkID(&fm, &opts.Type)
	out = append(out, checkStatus(&fm, &opts.Type)...)
	out = append(out, checkCreated(&fm)...)
	out = append(out, checkTitle(&fm)...)
	out = append(out, checkSchemaName(&fm)...)

	if opts.Filename != "" {
		out = append(out, checkFilename(&fm, opts.Filename)...)
	}

	return out
}

// checkID reports an id whose prefix or number does not match the type's
// configuration. Both are errors: the id is how every other tool in the
// fleet addresses the document, and `docz status set` resolves by it.
func checkID(fm *document.Frontmatter, typ *config.TypeConfig) []Finding {
	if fm.ID == "" {
		return []Finding{{
			Code:     codeIDPrefix,
			Severity: Error,
			Line:     frontmatterLine,
			Detail:   "document has no id",
		}}
	}

	m := idPattern.FindStringSubmatch(fm.ID)
	if m == nil {
		return []Finding{{
			Code:     codeIDPrefix,
			Severity: Error,
			Line:     frontmatterLine,
			Detail:   fmt.Sprintf("id %q is not a prefix and a number", fm.ID),
		}}
	}

	var out []Finding

	// Folded, because the prefix is a display choice: a repo that configures
	// "rfc" and writes "RFC-0001" has not made a mistake.
	if typ.IDPrefix != "" && !strings.EqualFold(m[1], typ.IDPrefix) {
		out = append(out, Finding{
			Code:     codeIDPrefix,
			Severity: Error,
			Line:     frontmatterLine,
			Detail: fmt.Sprintf("id prefix is %q, but this type uses %q",
				m[1], typ.IDPrefix),
		})
	}

	if typ.IDWidth > 0 && len(m[2]) != typ.IDWidth {
		out = append(out, Finding{
			Code:     "frontmatter.id-number",
			Severity: Error,
			Line:     frontmatterLine,
			Detail: fmt.Sprintf("id number %q is %d digits, but this type uses %d",
				m[2], len(m[2]), typ.IDWidth),
		})
	}

	return out
}

// checkStatus reports a status the type's configuration does not list.
//
// An empty status list means the type has not said what its statuses are,
// so nothing can be wrong. Comparison is folded, since a status is prose
// the author types.
func checkStatus(fm *document.Frontmatter, typ *config.TypeConfig) []Finding {
	if len(typ.Statuses) == 0 {
		return nil
	}

	if fm.Status == "" {
		return []Finding{{
			Code:     "frontmatter.status",
			Severity: Error,
			Line:     frontmatterLine,
			Detail: fmt.Sprintf("document has no status; this type allows %s",
				strings.Join(typ.Statuses, ", ")),
		}}
	}

	for _, allowed := range typ.Statuses {
		if strings.EqualFold(string(fm.Status), allowed) {
			return nil
		}
	}

	return []Finding{{
		Code:     "frontmatter.status",
		Severity: Error,
		Line:     frontmatterLine,
		Detail: fmt.Sprintf("status %q is not one of %s",
			fm.Status, strings.Join(typ.Statuses, ", ")),
	}}
}

// checkCreated reports a created date that is absent or not ISO. A warning:
// the date is metadata a reader uses to order documents, and getting it
// wrong costs nothing that cannot be fixed later.
func checkCreated(fm *document.Frontmatter) []Finding {
	if fm.Created == "" {
		return []Finding{{
			Code:     "frontmatter.created",
			Severity: Warning,
			Line:     frontmatterLine,
			Detail:   "document has no created date",
		}}
	}

	if _, err := time.Parse(time.DateOnly, strings.Trim(fm.Created, `"'`)); err != nil {
		return []Finding{{
			Code:     "frontmatter.created",
			Severity: Warning,
			Line:     frontmatterLine,
			Detail:   fmt.Sprintf("created date %q is not YYYY-MM-DD", fm.Created),
		}}
	}

	return nil
}

// checkTitle reports an empty title, or one still holding the template's
// placeholder.
func checkTitle(fm *document.Frontmatter) []Finding {
	if strings.TrimSpace(fm.Title) == "" {
		return []Finding{{
			Code:     "frontmatter.title",
			Severity: Warning,
			Line:     frontmatterLine,
			Detail:   "document has no title",
		}}
	}

	return nil
}

// checkSchemaName reports a schema name that is not [a-z0-9][a-z0-9_-]*
// (DESIGN-0015 §3). An empty field is the common case and means the
// document's own type, so it is not a finding.
//
// This is Document's schema.* finding. The sibling schema.unresolved
// belongs to the tier that resolves names to files, since Document only
// ever sees a Schema that has already been resolved.
func checkSchemaName(fm *document.Frontmatter) []Finding {
	if fm.Schema == "" || schemaNamePattern.MatchString(fm.Schema) {
		return nil
	}

	return []Finding{{
		Code:     "schema.name",
		Severity: Error,
		Line:     frontmatterLine,
		Detail: fmt.Sprintf("schema name %q is not lower-case letters, digits, hyphens, and underscores",
			fm.Schema),
	}}
}

// checkFilename reports a filename that does not follow the docz
// convention, or whose leading number disagrees with the frontmatter id.
//
// The disagreement matters because the two are used interchangeably: the
// README index links by filename and `docz status set` resolves by id, so a
// document where they differ is reachable two ways that do not lead to the
// same place.
func checkFilename(fm *document.Frontmatter, path string) []Finding {
	name := filepath.Base(path)

	if !document.IsDoczFile(name) {
		return []Finding{{
			Code:     "file.name",
			Severity: Error,
			Detail: fmt.Sprintf("filename %q is not a docz filename (0001-some-slug.md)",
				name),
		}}
	}

	m := idPattern.FindStringSubmatch(fm.ID)
	if m == nil {
		return nil
	}

	digits := document.DoczFilePattern.FindStringSubmatch(name)
	if digits == nil {
		return nil
	}

	fromID, errID := strconv.Atoi(m[2])
	fromName, errName := strconv.Atoi(digits[1])

	if errID != nil || errName != nil || fromID == fromName {
		return nil
	}

	return []Finding{{
		Code:     "file.name",
		Severity: Error,
		Detail: fmt.Sprintf("filename says document %d but the id says %d",
			fromName, fromID),
	}}
}
