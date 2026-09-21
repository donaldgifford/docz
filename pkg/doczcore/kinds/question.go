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

	// Recommended is true when the option carries the parenthesised
	// recommendation marker. The corpus qualifies it — "*(recommendation,
	// revised 2026-09-19)*", "*(recommendation, per review)*" — so the
	// marker is matched by its opening rather than as a fixed string.
	//
	// The fleet's convention also makes option "a" the recommendation, but
	// that is a habit rather than the contract: an author who recommends
	// "c" says so, and reading the marker rather than the letter is what
	// lets them.
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
	// Number is the number from the heading, reported as written. A heading
	// that carries no number is not a question and is not reported at all,
	// but a heading numbered zero is reported as zero rather than dropped:
	// the grammar numbers from 1, so that is validate's finding to make, and
	// a reader that filtered it would hide the document's mistake.
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
	//
	// The letter may be wrapped in emphasis. The fleet writes options both
	// ways — "- a. Keep pkg/" and "- **a. (Recommendation) Out of scope**" —
	// and 129 of the corpus's 607 option bullets use the second spelling. An
	// anchored letter missed all of them, and missed them *silently*: a
	// question mixing the two reported the plain options and dropped the rest
	// with no finding, because open-questions.no-options only fires when a
	// question has none at all.
	optionBullet = regexp.MustCompile(`^([*_]{0,2})([a-zA-Z])[.)]\s+(.*)$`)

	// resolvedQuote matches the resolution blockquote's opening. The date
	// and the letter are both optional so a malformed blockquote still
	// reports as a resolution: an author who wrote "Resolved: (a)" has
	// resolved the question, and reporting it as open would be worse.
	//
	// The letter must be in parentheses. The corpus resolves some questions
	// without one — "Resolved 2026-09-19: superseded by DESIGN-0015" — and
	// an unparenthesised letter would read the "s" of "superseded" as the
	// choice. The opening bold need not be closed on the same line, because
	// two of the corpus's resolutions close it two lines later.
	resolvedQuote = regexp.MustCompile(
		`^\*{0,2}Resolved\*{0,2}\s*(\d{4}-\d{2}-\d{2})?\*{0,2}\s*:?\s*(?:\(([a-zA-Z])\))?`,
	)

	// recommendationMarker matches the marker and any qualification inside
	// its parentheses.
	//
	// The emphasis is optional and the word is matched case-insensitively,
	// because the corpus is split almost evenly and the stricter spelling is
	// the minority: 72 options write "*(recommendation)*" and 95 write a bare
	// "(Recommendation)", usually inside the option's own bold. Requiring the
	// stars reported the majority as unrecommended.
	//
	// The word must open the parenthesis, so an option whose prose says
	// "(not the recommendation we took)" is not a recommendation.
	recommendationMarker = regexp.MustCompile(
		`(?i)\*{0,2}\((?:recommendation|recommended)[^)]*\)\*{0,2}`,
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
			Line:     h.Line + bodyOffset,
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

		text := strings.TrimSpace(m[3])

		// An option whose letter opened with emphasis usually closes it at
		// the end of the option, so consuming the opener alone would leave
		// the text ending in a marker that now opens nothing. Only a
		// matching closer is removed: text that legitimately ends in bold
		// keeps it.
		if m[1] != "" {
			text = strings.TrimSpace(strings.TrimSuffix(text, m[1]))
		}

		out = append(out, Option{
			Letter:      strings.ToLower(m[2]),
			Text:        text,
			Recommended: recommendationMarker.MatchString(text),
			Line:        it.Line + bodyOffset,
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

		// The note is the rest of the blockquote, folded. A bare ">" is a
		// paragraph break inside one blockquote, not its end: DESIGN-0014's
		// tenth question records its amendment in a second paragraph of the
		// same quote, and dropping it would lose the amendment.
		for k := n + 1; k <= through && k <= len(lines); k++ {
			more, ok := blockquoteText(lines[k-1])
			if !ok {
				break
			}

			if trimmed := strings.TrimSpace(stripComments(more)); trimmed != "" {
				parts = append(parts, trimmed)
			}
		}

		return &Resolution{
			Date:   m[1],
			Choice: strings.ToLower(m[2]),
			Note:   resolutionNote(strings.Join(parts, " ")),
			Line:   n + bodyOffset,
		}
	}

	return nil
}

// resolutionNote cleans up what is left of a resolution blockquote once the
// date and the choice have been read.
//
// Bold markers go. The corpus wraps the whole resolution sentence in bold
// and closes it anywhere from just after the letter to two lines later, so
// the opener is consumed by the pattern and the closer would otherwise land
// in the middle of the note. Dropping both keeps the words and loses only
// the emphasis, which a note has no use for.
//
// Only leading punctuation is trimmed. A trailing period ends a sentence,
// and cutting it would make every note read as a fragment.
func resolutionNote(s string) string {
	return strings.TrimSpace(strings.TrimLeft(strings.ReplaceAll(s, "**", ""), " .,:;—-"))
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
