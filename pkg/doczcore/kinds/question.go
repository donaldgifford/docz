package kinds

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// Option is one lettered choice under an open question.
type Option struct {
	// Letter is the option's letter, lower-cased.
	Letter string

	// Text is the option, wrapped lines folded, inline markdown kept.
	Text string

	// Recommended is true when the option carries the "*(recommendation)*"
	// marker. The fleet's convention also makes option "a" the
	// recommendation, but that is a habit rather than the contract: an
	// author who recommends "c" says so, and reading the marker rather
	// than the letter is what lets them.
	Recommended bool

	// Line is the 1-based line of the bullet, counted from the start of
	// the region.
	Line int
}

// Resolution is the blockquote that records how an open question was
// settled.
type Resolution struct {
	// Date is the date as written, normally ISO ("2026-09-20"), reported
	// verbatim rather than parsed: a reader that wants a time.Time knows
	// its own layout, and a malformed date should not lose the choice.
	Date string

	// Choice is the letter chosen, lower-cased, or "" when the blockquote
	// names none.
	Choice string

	// Note is the rest of the blockquote: the reasoning, folded to one
	// line.
	Note string

	// Line is the 1-based line the blockquote opens on, counted from the
	// start of the region.
	Line int
}

// Question is one open question: a numbered heading, its lettered options,
// and its resolution when it has one.
type Question struct {
	// Number is the number from the heading. A heading that is not
	// numbered is not a question and is not reported.
	Number int

	// Title is the heading text after the number, inline markdown
	// stripped.
	Title string

	// Options are the lettered bullets under the heading, in document
	// order.
	Options []Option

	// Resolved is the resolution blockquote, or nil while the question is
	// open. A pointer rather than a bool-and-value pair because "resolved"
	// and "what it resolved to" are one fact.
	Resolved *Resolution

	// Line is the 1-based line of the heading, counted from the start of
	// the region.
	Line int
}

var (
	// questionHeading matches "1." or "1)" at the head of a level-3
	// heading, the shape the fleet numbers its open questions with.
	questionHeading = regexp.MustCompile(`^(\d{1,3})[.)]\s*(.*)$`)

	// optionBullet matches a lettered option bullet: "a." or "a)".
	optionBullet = regexp.MustCompile(`^([a-zA-Z])[.)]\s+(.*)$`)

	// resolvedQuote matches the resolution blockquote's opening. The date
	// and the letter are both optional in the pattern so a malformed
	// blockquote still reports as a resolution: an author who wrote
	// "Resolved: (a)" has resolved the question, and reporting it as open
	// would be the worse answer.
	resolvedQuote = regexp.MustCompile(
		`^\*{0,2}Resolved\*{0,2}\s*([0-9]{4}-[0-9]{2}-[0-9]{2})?\s*:?\s*\(?([a-zA-Z])?\)?`,
	)
)

// OpenQuestions returns the region's numbered questions with their options
// and resolutions.
//
// The grammar is the fleet's, and this is its only definition (DESIGN-0014
// §2.12): level-3 headings numbered from 1, options as lettered bullets,
// and a resolution as a blockquote opening "**Resolved <date>: (<letter>)".
// A question with no options is still a question — the corpus has them —
// and so is one whose numbering skips, because renumbering is the author's
// job and validate's finding, not a reason to drop the question.
func OpenQuestions(region []byte) []Question {
	lines := bodyLines(region)
	body := strings.Join(lines, "\n")

	var heads []docparse.Heading

	for _, h := range docparse.Headings([]byte(body)) {
		if h.Level == 3 {
			heads = append(heads, h)
		}
	}

	if len(heads) == 0 {
		return nil
	}

	folded := foldLines(lines)

	out := make([]Question, 0, len(heads))

	for i, h := range heads {
		m := questionHeading.FindStringSubmatch(h.Text)
		if m == nil {
			continue
		}

		number, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}

		end := len(lines)
		if i+1 < len(heads) {
			end = heads[i+1].Line - 1
		}

		out = append(out, Question{
			Number:   number,
			Title:    strings.TrimSpace(m[2]),
			Options:  optionsIn(folded, h.Line, end),
			Resolved: resolutionIn(lines, h.Line, end),
			Line:     h.Line,
		})
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

// optionsIn collects the lettered top-level bullets between two lines.
func optionsIn(folded []foldedItem, after, through int) []Option {
	var out []Option

	for _, it := range folded {
		if it.Indent != 0 || it.Line <= after || it.Line > through {
			continue
		}

		m := optionBullet.FindStringSubmatch(it.Text)
		if m == nil {
			continue
		}

		text := strings.TrimSpace(m[2])

		out = append(out, Option{
			Letter:      strings.ToLower(m[1]),
			Text:        text,
			Recommended: strings.Contains(text, "*(recommendation)*"),
			Line:        it.Line,
		})
	}

	return out
}

// resolutionIn finds the resolution blockquote between two lines. The note
// is the rest of the blockquote, folded: a resolution runs to several lines
// in the corpus and the reasoning is the useful part of it.
func resolutionIn(lines []string, after, through int) *Resolution {
	for n := after + 1; n <= through && n <= len(lines); n++ {
		quoted, ok := blockquoteText(lines[n-1])
		if !ok {
			continue
		}

		m := resolvedQuote.FindStringSubmatch(quoted)
		if m == nil {
			continue
		}

		parts := []string{strings.TrimSpace(stripComments(quoted[len(m[0]):]))}

		for k := n + 1; k <= through && k <= len(lines); k++ {
			more, ok := blockquoteText(lines[k-1])
			if !ok || strings.TrimSpace(more) == "" {
				break
			}

			parts = append(parts, strings.TrimSpace(stripComments(more)))
		}

		return &Resolution{
			Date:   m[1],
			Choice: strings.ToLower(m[2]),
			Note:   strings.TrimSpace(strings.Trim(strings.Join(parts, " "), " .—-")),
			Line:   n,
		}
	}

	return nil
}

// blockquoteText strips a leading ">" and returns the text after it.
func blockquoteText(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)

	rest, ok := strings.CutPrefix(trimmed, ">")
	if !ok {
		return "", false
	}

	return strings.TrimSpace(rest), true
}
