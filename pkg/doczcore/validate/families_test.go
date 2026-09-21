package validate_test

import (
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/validate"
)

// rfcType is a realistic TypeConfig to check frontmatter against, so the
// frontmatter findings are driven by configuration rather than by a default
// this package invented.
func rfcType() config.TypeConfig {
	return config.DefaultConfig().Types["rfc"]
}

// goodFrontmatter is a frontmatter block with nothing wrong with it, so a
// case that is about some other family does not trip a frontmatter finding on
// the way past.
const goodFrontmatter = "---\nid: RFC-0001\ntitle: \"A title\"\n" +
	"status: Draft\nauthor: A\ncreated: 2026-09-20\n---\n\n# RFC-0001: A title\n\n"

// TestDocument_CodeFamilies is one passing and one failing document per code
// family (DESIGN-0015 §4). The Code is the contract, so each case asserts on
// the code rather than on the wording, which is the same discipline a
// consumer is told to use.
func TestDocument_CodeFamilies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		opts     validate.Options
		wantCode string // "" means the case must report nothing
		wantSev  validate.Severity
		notCodes []string // codes that must not appear alongside
	}{
		// file.*
		{
			name:     "file.crlf: a carriage return",
			body:     goodFrontmatter + "text\r\n",
			wantCode: "file.crlf",
			wantSev:  validate.Error,
		},
		{
			name: "file.crlf: LF only passes",
			body: goodFrontmatter + "text\n",
		},
		{
			name:     "file.name: not a docz filename",
			body:     goodFrontmatter,
			opts:     validate.Options{Filename: "notes.md"},
			wantCode: "file.name",
			wantSev:  validate.Error,
		},
		{
			name:     "file.name: the number disagrees with the id",
			body:     goodFrontmatter,
			opts:     validate.Options{Filename: "0007-a-title.md"},
			wantCode: "file.name",
			wantSev:  validate.Error,
		},
		{
			name: "file.name: the number agrees with the id",
			body: goodFrontmatter,
			opts: validate.Options{Filename: "0001-a-title.md"},
		},

		// frontmatter.*
		{
			name:     "frontmatter.missing: no block at all",
			body:     "# A title\n\ntext\n",
			wantCode: "frontmatter.missing",
			wantSev:  validate.Error,
			// One finding for one absence, not six.
			notCodes: []string{
				"frontmatter.id-prefix", "frontmatter.status",
				"frontmatter.created", "frontmatter.title",
			},
		},
		{
			name:     "frontmatter.id-prefix: the wrong prefix",
			body:     "---\nid: ADR-0001\nstatus: Draft\ncreated: 2026-09-20\ntitle: T\n---\n",
			opts:     validate.Options{Type: rfcType()},
			wantCode: "frontmatter.id-prefix",
			wantSev:  validate.Error,
		},
		{
			name:     "frontmatter.id-number: the wrong width",
			body:     "---\nid: RFC-01\nstatus: Draft\ncreated: 2026-09-20\ntitle: T\n---\n",
			opts:     validate.Options{Type: rfcType()},
			wantCode: "frontmatter.id-number",
			wantSev:  validate.Error,
		},
		{
			name:     "frontmatter.status: a status the type does not allow",
			body:     "---\nid: RFC-0001\nstatus: Shipped\ncreated: 2026-09-20\ntitle: T\n---\n",
			opts:     validate.Options{Type: rfcType()},
			wantCode: "frontmatter.status",
			wantSev:  validate.Error,
		},
		{
			name: "frontmatter.status: folded, so a case difference is not a finding",
			body: "---\nid: RFC-0001\nstatus: draft\ncreated: 2026-09-20\ntitle: T\n---\n",
			opts: validate.Options{Type: rfcType()},
		},
		{
			name:     "frontmatter.created: not a date",
			body:     "---\nid: RFC-0001\nstatus: Draft\ncreated: last Tuesday\ntitle: T\n---\n",
			opts:     validate.Options{Type: rfcType()},
			wantCode: "frontmatter.created",
			wantSev:  validate.Warning,
		},
		{
			name:     "frontmatter.title: an empty title",
			body:     "---\nid: RFC-0001\nstatus: Draft\ncreated: 2026-09-20\ntitle: \"\"\n---\n",
			opts:     validate.Options{Type: rfcType()},
			wantCode: "frontmatter.title",
			wantSev:  validate.Warning,
		},

		// schema.name
		{
			name: "schema.name: not the name grammar",
			body: "---\nid: RFC-0001\nstatus: Draft\ncreated: 2026-09-20\ntitle: T\n" +
				"schema: Impl Strict\n---\n",
			opts:     validate.Options{Type: rfcType()},
			wantCode: "schema.name",
			wantSev:  validate.Error,
		},
		{
			name: "schema.name: a valid name",
			body: "---\nid: RFC-0001\nstatus: Draft\ncreated: 2026-09-20\ntitle: T\n" +
				"schema: impl-strict_2\n---\n",
			opts: validate.Options{Type: rfcType()},
		},

		// marker.*
		{
			name:     "marker.stray-end: an end with no start",
			body:     goodFrontmatter + "<!--docz:summary:end-->\n",
			wantCode: "marker.stray-end",
			wantSev:  validate.Error,
		},
		{
			name:     "marker.unclosed: a start never closed",
			body:     goodFrontmatter + "<!--docz:summary:start-->\n## Summary\n",
			wantCode: "marker.unclosed",
			wantSev:  validate.Error,
		},
		{
			name: "marker.spelling: a lenient spelling is read and reported",
			body: goodFrontmatter +
				"<!-- docz : summary : start -->\n## Summary\n\ntext\n<!--docz:summary:end-->\n",
			wantCode: "marker.spelling",
			wantSev:  validate.Warning,
			// Read, not skipped: no region is missing and no marker is stray.
			notCodes: []string{"marker.stray-end", "marker.unclosed"},
		},
		{
			name:     "marker.in-fence: a marker shown in an example",
			body:     goodFrontmatter + "```markdown\n<!--docz:summary:start-->\n```\n",
			wantCode: "marker.in-fence",
			wantSev:  validate.Warning,
			notCodes: []string{"marker.unclosed"},
		},

		// region.*
		{
			name:     "region.missing: a schema kind the document lacks",
			body:     goodFrontmatter + "## Summary\n\ntext\n",
			opts:     validate.Options{Schema: schemaOf("summary")},
			wantCode: "region.missing",
			wantSev:  validate.Error,
		},
		{
			name: "region.duplicate-singleton: two summaries",
			body: goodFrontmatter +
				"<!--docz:summary:start-->\n## Summary\n\none\n<!--docz:summary:end-->\n" +
				"<!--docz:summary:start-->\n## Summary\n\ntwo\n<!--docz:summary:end-->\n",
			wantCode: "region.duplicate-singleton",
			wantSev:  validate.Warning,
		},
		{
			name: "region.wrong-parent: tasks outside a phase",
			body: goodFrontmatter +
				"<!--docz:tasks:start-->\n#### Tasks\n\n- [ ] a\n<!--docz:tasks:end-->\n",
			opts: validate.Options{Schema: validate.Schema{Regions: []validate.SchemaRegion{
				{Kind: "tasks", Parent: "phase"},
			}}},
			wantCode: "region.wrong-parent",
			wantSev:  validate.Error,
		},
		{
			name: "region.inferred: an unmarked document with a heading spec",
			body: goodFrontmatter + "## Summary\n\ntext\n",
			opts: validate.Options{
				Headings: kinds.HeadingSpec{{Kind: "summary", Level: 2, Text: "summary"}},
			},
			wantCode: "region.inferred",
			wantSev:  validate.Warning,
		},
		{
			name: "region.inferred: not reported once the document is marked",
			body: goodFrontmatter +
				"<!--docz:summary:start-->\n## Summary\n\ntext\n<!--docz:summary:end-->\n",
			opts: validate.Options{
				Headings: kinds.HeadingSpec{{Kind: "summary", Level: 2, Text: "summary"}},
			},
		},

		// references.*
		{
			name: "references.no-link: a bullet with no link",
			body: goodFrontmatter +
				"<!--docz:references:start-->\n## References\n\n- ADR-0002, the decision\n" +
				"<!--docz:references:end-->\n",
			wantCode: "references.no-link",
			wantSev:  validate.Warning,
		},
		{
			name: "references.no-link: a bullet with a link",
			body: goodFrontmatter +
				"<!--docz:references:start-->\n## References\n\n- [ADR-0002](../adr/0002.md)\n" +
				"<!--docz:references:end-->\n",
		},

		// open-questions.*
		{
			name: "open-questions.numbering: a repeat",
			body: goodFrontmatter +
				"<!--docz:open-questions:start-->\n## Open Questions\n\n" +
				"### 1. First?\n\n- a. Yes.\n\n### 1. Also first?\n\n- a. Yes.\n" +
				"<!--docz:open-questions:end-->\n",
			wantCode: "open-questions.numbering",
			wantSev:  validate.Warning,
		},
		{
			name: "open-questions.numbering: a reversal",
			body: goodFrontmatter +
				"<!--docz:open-questions:start-->\n## Open Questions\n\n" +
				"### 2. Second?\n\n- a. Yes.\n\n### 1. First?\n\n- a. Yes.\n" +
				"<!--docz:open-questions:end-->\n",
			wantCode: "open-questions.numbering",
			wantSev:  validate.Warning,
		},
		{
			// A gap is what a document has after it resolves questions and
			// keeps one for its alternatives, which the corpus does. It used
			// to be a finding; reporting it told a document off for being
			// tidy.
			name: "open-questions: a gap is legitimate",
			body: goodFrontmatter +
				"<!--docz:open-questions:start-->\n## Open Questions\n\n" +
				"### 1. First?\n\n- a. Yes.\n\n### 3. Third?\n\n- a. Yes.\n" +
				"<!--docz:open-questions:end-->\n",
		},
		{
			// The spelling 129 of the corpus's 607 option bullets use. The
			// letter is inside the emphasis, and an anchored match dropped
			// the option silently.
			name: "open-questions: an option whose letter is emphasised counts",
			body: goodFrontmatter +
				"<!--docz:open-questions:start-->\n## Open Questions\n\n" +
				"### 1. First?\n\n- **a. (Recommendation) Yes.**\n" +
				"<!--docz:open-questions:end-->\n",
		},
		{
			name: "open-questions.no-options: a question with none",
			body: goodFrontmatter +
				"<!--docz:open-questions:start-->\n## Open Questions\n\n### 1. First?\n\nprose\n" +
				"<!--docz:open-questions:end-->\n",
			wantCode: "open-questions.no-options",
			wantSev:  validate.Warning,
		},
		{
			name: "open-questions: numbered from one with options passes",
			body: goodFrontmatter +
				"<!--docz:open-questions:start-->\n## Open Questions\n\n" +
				"### 1. First?\n\n- a. Yes.\n\n### 2. Second?\n\n- a. Yes.\n" +
				"<!--docz:open-questions:end-->\n",
		},

		// tasks.*
		{
			name: "tasks.not-checkbox: a plain bullet in a tasks region",
			body: goodFrontmatter +
				"<!--docz:tasks:start-->\n#### Tasks\n\n- [ ] a real task\n- a plain bullet\n" +
				"<!--docz:tasks:end-->\n",
			wantCode: "tasks.not-checkbox",
			wantSev:  validate.Error,
		},
		{
			name: "tasks.nested-checkbox: a nested one is not a task",
			body: goodFrontmatter +
				"<!--docz:tasks:start-->\n#### Tasks\n\n- [ ] a real task\n  - [ ] nested\n" +
				"<!--docz:tasks:end-->\n",
			wantCode: "tasks.nested-checkbox",
			wantSev:  validate.Warning,
		},
		{
			name: "tasks: checkboxes at the top level pass",
			body: goodFrontmatter +
				"<!--docz:tasks:start-->\n#### Tasks\n\n- [ ] one\n- [x] two\n" +
				"<!--docz:tasks:end-->\n",
		},

		// toc.*
		{
			name:     "toc.missing: the schema wants one and there is none",
			body:     goodFrontmatter + "## Summary\n\ntext\n",
			opts:     validate.Options{Schema: schemaOf("toc")},
			wantCode: "toc.missing",
			wantSev:  validate.Warning,
			// The specific finding supersedes the generic one.
			notCodes: []string{"region.missing"},
		},
		{
			name: "toc.stale: a generated list that no longer matches",
			body: goodFrontmatter +
				"<!--toc:start-->\n- [Gone](#gone)\n<!--toc:end-->\n\n" +
				"## Alpha\n\n## Beta\n\n## Gamma\n",
			opts:     validate.Options{MinHeadings: 1},
			wantCode: "toc.stale",
			wantSev:  validate.Warning,
		},
		{
			name: "toc: an empty region is not yet generated, not stale",
			body: goodFrontmatter +
				"<!--toc:start-->\n<!--toc:end-->\n\n## Alpha\n\n## Beta\n",
			opts: validate.Options{MinHeadings: 1},
		},
		{
			name: "toc: a fresh list passes",
			body: goodFrontmatter +
				"<!--toc:start-->\n- [Alpha](#alpha)\n- [Beta](#beta)\n<!--toc:end-->\n\n" +
				"## Alpha\n\n## Beta\n",
			opts: validate.Options{MinHeadings: 1},
		},
		{
			// An unmarked document whose ToC pair is real and filled. The
			// ToC lookup has to read the document's own markers, because
			// inference replaces the region list wholesale and a ToC pair
			// has no heading to be inferred from -- so the real pair used
			// to vanish and this reported toc.missing on a good document.
			name: "toc: an inferred document keeps its real ToC pair",
			body: goodFrontmatter +
				"<!--toc:start-->\n- [Summary](#summary)\n<!--toc:end-->\n\n" +
				"## Summary\n\ntext\n",
			opts: validate.Options{
				Schema: schemaOf("toc", "summary"),
				Headings: kinds.HeadingSpec{
					{Kind: "summary", Level: 2, Text: "summary"},
				},
				MinHeadings: 1,
			},
			// The inference warning is the expected one. What must not
			// appear is any complaint about the ToC or the summary region.
			wantCode: "region.inferred",
			wantSev:  validate.Warning,
			notCodes: []string{"toc.missing", "toc.stale", "region.missing"},
		},

		// content.*
		{
			name: "content.not-bullets: prose where the kind is a list",
			body: goodFrontmatter +
				"<!--docz:goals:start-->\n### Goals\n\nWe want the thing to work.\n" +
				"<!--docz:goals:end-->\n",
			wantCode: "content.not-bullets",
			wantSev:  validate.Warning,
		},
		{
			name: "content.not-bullets: an empty section is incomplete, not malformed",
			body: goodFrontmatter +
				"<!--docz:goals:start-->\n### Goals\n\n-\n<!--docz:goals:end-->\n",
		},
		{
			name: "content.not-ordered: an approach as bullets",
			body: goodFrontmatter +
				"<!--docz:approach:start-->\n## Approach\n\n- do this\n- then that\n" +
				"<!--docz:approach:end-->\n",
			wantCode: "content.not-ordered",
			wantSev:  validate.Warning,
		},
		{
			name: "content.not-table: prose where the kind is a table",
			body: goodFrontmatter +
				"<!--docz:risks:start-->\n## Risks and Mitigations\n\nNothing much.\n" +
				"<!--docz:risks:end-->\n",
			wantCode: "content.no-table",
			wantSev:  validate.Warning,
		},
		{
			name: "content.table-columns: a table missing a column",
			body: goodFrontmatter +
				"<!--docz:risks:start-->\n## Risks and Mitigations\n\n" +
				"| Risk | Impact |\n| ---- | ------ |\n| a | b |\n<!--docz:risks:end-->\n",
			wantCode: "content.table-columns",
			wantSev:  validate.Warning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := validate.Document([]byte(tt.body), tt.opts)

			codes := make([]string, 0, len(got))
			for _, f := range got {
				codes = append(codes, f.Code)
			}

			if tt.wantCode == "" {
				if len(got) > 0 {
					t.Errorf("expected no findings, got %v", got)
				}

				return
			}

			found := false

			for _, f := range got {
				if f.Code != tt.wantCode {
					continue
				}

				found = true

				if f.Severity != tt.wantSev {
					t.Errorf("%s severity = %s, want %s", f.Code, f.Severity, tt.wantSev)
				}

				if f.Detail == "" {
					t.Errorf("%s has no detail", f.Code)
				}
			}

			if !found {
				t.Errorf("expected %s, got %v", tt.wantCode, codes)
			}

			for _, unwanted := range tt.notCodes {
				if strings.Contains(strings.Join(codes, " "), unwanted) {
					t.Errorf("did not expect %s alongside %s: %v",
						unwanted, tt.wantCode, codes)
				}
			}
		})
	}
}

// schemaOf builds a schema requiring the given top-level kinds.
func schemaOf(kindNames ...string) validate.Schema {
	regions := make([]validate.SchemaRegion, 0, len(kindNames))
	for _, kind := range kindNames {
		regions = append(regions, validate.SchemaRegion{Kind: kind})
	}

	return validate.Schema{Regions: regions}
}

// Findings come back in the order a reader walks the document, with the ones
// about the document as a whole first. A consumer prints them in order, so an
// unstable order makes every run's output a different diff.
func TestDocument_FindingOrder(t *testing.T) {
	t.Parallel()

	body := goodFrontmatter +
		"<!--docz:references:start-->\n## References\n\n- no link here\n- nor here\n" +
		"<!--docz:references:end-->\n"

	got := validate.Document([]byte(body), validate.Options{Schema: schemaOf("summary")})

	if len(got) < 3 {
		t.Fatalf("expected at least three findings, got %v", got)
	}

	if got[0].Line != 0 || got[0].Code != "region.missing" {
		t.Errorf("first finding = %s, want the document-level region.missing", got[0])
	}

	for i := 1; i < len(got); i++ {
		if got[i].Line < got[i-1].Line {
			t.Errorf("findings out of line order: %s after %s", got[i], got[i-1])
		}
	}
}
