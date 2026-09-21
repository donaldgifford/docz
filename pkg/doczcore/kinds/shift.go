package kinds

import "github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"

// Every reader in this package numbers lines from the start of the region it
// was handed. A type package's Doc numbers them from the start of the
// document, because a Line there is an address a consumer acts on: the line
// docwrite splices at, the line an editor jumps to (DESIGN-0014 §5).
//
// The helpers below are that conversion, and they live here rather than in
// each type package because the package that decides the numbering should own
// changing it. Five copies of the same eight lines would be five chances for
// one of them to be off by one, which is a mistake that looks like a line
// number.
//
// One function per value type rather than one generic: Line sits at a
// different depth in each — a question carries lines on its options and its
// resolution too — so the shapes are not actually the same operation.
//
// docparse.TaskItem has no helper here. It belongs to the facts layer, and
// only one type package has a field of them.

// ShiftItems returns the items with their lines rebased onto the document the
// region came from.
func ShiftItems(items []Item, at docparse.Region) []Item {
	if len(items) == 0 {
		return nil
	}

	out := make([]Item, 0, len(items))

	for _, item := range items {
		item.Line += at.Start
		out = append(out, item)
	}

	return out
}

// ShiftReferences rebases reference lines onto the document.
func ShiftReferences(refs []Reference, at docparse.Region) []Reference {
	if len(refs) == 0 {
		return nil
	}

	out := make([]Reference, 0, len(refs))

	for _, ref := range refs {
		ref.Line += at.Start
		out = append(out, ref)
	}

	return out
}

// ShiftCriteria rebases criterion lines onto the document.
func ShiftCriteria(criteria []Criterion, at docparse.Region) []Criterion {
	if len(criteria) == 0 {
		return nil
	}

	out := make([]Criterion, 0, len(criteria))

	for _, c := range criteria {
		c.Line += at.Start
		out = append(out, c)
	}

	return out
}

// ShiftAlternatives rebases alternative lines onto the document.
func ShiftAlternatives(alts []Alternative, at docparse.Region) []Alternative {
	if len(alts) == 0 {
		return nil
	}

	out := make([]Alternative, 0, len(alts))

	for _, alt := range alts {
		alt.Line += at.Start
		out = append(out, alt)
	}

	return out
}

// ShiftDecisions rebases decision-row lines onto the document.
func ShiftDecisions(decisions []Decision, at docparse.Region) []Decision {
	if len(decisions) == 0 {
		return nil
	}

	out := make([]Decision, 0, len(decisions))

	for _, d := range decisions {
		d.Line += at.Start
		out = append(out, d)
	}

	return out
}

// ShiftSections rebases section lines onto the document.
func ShiftSections(sections []Section, at docparse.Region) []Section {
	if len(sections) == 0 {
		return nil
	}

	out := make([]Section, 0, len(sections))

	for _, s := range sections {
		s.Line += at.Start
		out = append(out, s)
	}

	return out
}

// ShiftQuestions rebases an open question's lines onto the document, including
// the ones on its options and its resolution: a consumer that jumps to an
// unresolved option needs that line, not the heading above it.
func ShiftQuestions(questions []Question, at docparse.Region) []Question {
	if len(questions) == 0 {
		return nil
	}

	out := make([]Question, 0, len(questions))

	for _, q := range questions {
		q.Line += at.Start

		if len(q.Options) > 0 {
			options := make([]Option, 0, len(q.Options))

			for _, option := range q.Options {
				option.Line += at.Start
				options = append(options, option)
			}

			q.Options = options
		}

		if q.Resolved != nil {
			resolved := *q.Resolved
			resolved.Line += at.Start
			q.Resolved = &resolved
		}

		out = append(out, q)
	}

	return out
}
