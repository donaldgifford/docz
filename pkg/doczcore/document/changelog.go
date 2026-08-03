package document

import (
	"errors"
	"regexp"
	"strings"
)

// ErrNoVersions is returned by ParseChangelog when the input contains no
// version headings — the file is not a changelog, or has no releases yet.
// Callers decide skip versus fail, mirroring ErrNoFrontmatter.
//
// On this error the input itself is the preamble; callers that want to
// render it already hold the bytes.
var ErrNoVersions = errors.New("no version headings found")

// Changelog is a parsed git-cliff / Keep-a-Changelog document.
//
// It is not a lossless representation of the input: text that is neither
// a heading nor part of a bullet item — column-0 prose between bullets,
// for instance — is discarded.
type Changelog struct {
	// Preamble is every byte before the first version heading (title and
	// intro prose), verbatim, including the newline that ends the last
	// preamble line. It is empty when the document opens with a version
	// heading.
	Preamble string

	// Versions in document order — git-cliff emits newest first.
	Versions []ChangelogVersion
}

// ChangelogVersion is one release section of a changelog.
type ChangelogVersion struct {
	// Version is the bare version string: brackets stripped, surrounding
	// whitespace trimmed, and a single leading "v" or "V" removed when a
	// digit follows it, so "[v0.4.2]" and "[0.4.2]" both yield "0.4.2".
	// An unreleased section is canonicalized to lowercase "unreleased",
	// whatever case the heading used.
	Version string

	// Unreleased reports whether the bracket content was "unreleased",
	// case-insensitively. It is true exactly when Version ==
	// "unreleased".
	Unreleased bool

	// Date is the heading text after the version bracket, with one
	// leading dash separator removed and whitespace trimmed — normally
	// "YYYY-MM-DD". It is not validated as a date, and is always empty
	// for an unreleased section.
	Date string

	// Groups in document order.
	Groups []ChangelogGroup
}

// ChangelogGroup is one "### Title" section within a version — a
// git-cliff commit group such as "Bug Fixes".
type ChangelogGroup struct {
	// Title is the heading text verbatim, inline markdown included.
	// Bullet items that appear inside a version before any group heading
	// are collected into a group with an empty Title. Only bullets are —
	// column-0 prose in that position is discarded like any other, so an
	// empty-Title group never holds loose text (see Changelog).
	Title string

	// Items holds one entry per top-level bullet, in document order.
	// The bullet marker is stripped from the first line; everything
	// indented under it — wrapped prose and nested sub-bullets, their
	// own markers intact — is kept verbatim, joined by newlines. To
	// render an item as markdown, re-prefix it with "- ".
	Items []string
}

// changelogVersionRE matches a version heading anchored at column 0:
// exactly two hashes, whitespace, then a bracketed version. Everything
// after the closing bracket is captured raw for the caller to split into
// a separator and a date, so a heading with an unusual separator still
// parses rather than vanishing.
//
// The column-0 anchor is a deliberate divergence from docparse.Headings,
// which trims each line before matching: here an indented "### ..." is
// content nested under a bullet, not a heading (see ChangelogGroup.Items).
var changelogVersionRE = regexp.MustCompile(`^##[ \t]+\[([^\]\n]*)\][ \t]*(.*)$`)

// changelogGroupRE matches a group heading anchored at column 0. A bare
// "###" with no title is a group with an empty Title.
var changelogGroupRE = regexp.MustCompile(`^###(?:[ \t]+(.*))?$`)

// changelogBulletRE matches a top-level bullet: a "-" or "*" at column 0
// followed by whitespace or end of line. Requiring that separator keeps
// "---" thematic breaks and "**bold**" text from opening an item.
var changelogBulletRE = regexp.MustCompile(`^[-*]([ \t]+|$)`)

// ParseChangelog parses git-cliff / Keep-a-Changelog markdown into
// versions, commit groups, and items.
//
// It returns either a non-nil *Changelog with a nil error, or nil and
// ErrNoVersions; there is no other error and no partial result. Parsing
// never panics — non-conforming markdown is parsed best-effort. When the
// error is nil, Versions is never empty.
//
// Headings are recognized only at column 0 and only inside brackets:
// "## [1.2.3] - 2026-01-01" and "## [unreleased]" open versions, "###
// Title" opens a group. Heading-shaped lines inside fenced code blocks
// are ignored (the fence rule matches the rest of the module: a trimmed
// line starting with ``` toggles, ~~~ does not).
//
// Duplicate version headings produce separate entries in document order:
// the parser reports what the file says rather than editorializing. The
// same holds for duplicate group titles.
//
// Line endings: input is expected to use LF. CRLF input parses to the
// same field values — a trailing carriage return is trimmed from
// recognized lines — but Preamble stays byte-verbatim.
func ParseChangelog(content []byte) (*Changelog, error) {
	p := changelogParser{src: string(content)}
	p.run()

	if len(p.out.Versions) == 0 {
		return nil, ErrNoVersions
	}
	// Copy out rather than returning &p.out: an interior pointer into the
	// parser keeps the whole changelogParser alive, and with it the
	// stringified source and the item buffer grown to the largest item.
	out := p.out
	return &out, nil
}

// changelogParser walks the document once, accumulating the open item,
// group, and version. The flush* methods commit whatever is open, so the
// walk itself stays a flat dispatch instead of a nest of conditionals.
type changelogParser struct {
	src string
	out Changelog

	inFence bool
	started bool // a version heading has been seen

	version     ChangelogVersion
	versionOpen bool
	group       ChangelogGroup

	// item accumulates the open item's text. It is a byte slice rather
	// than a string because a bullet's continuation is appended line by
	// line: concatenating would copy the whole item every line, making a
	// single long item quadratic in its size. (A strings.Builder would
	// also fix that, but it panics if the struct holding it is ever
	// copied, and ParseChangelog promises never to panic.)
	item      []byte
	itemOpen  bool
	itemBlank int // blank lines buffered after the open item
}

func (p *changelogParser) run() {
	for off := 0; off < len(p.src); {
		line, next := changelogLine(p.src, off)

		if isChangelogFence(line) {
			p.inFence = !p.inFence
			// The fence line is content, not a delimiter to drop: a
			// fence opened inside an item's continuation belongs to
			// that item.
			p.appendContinuation(line)
			off = next
			continue
		}

		if p.inFence {
			p.appendContinuation(line)
			off = next
			continue
		}

		p.classify(line, off)
		off = next
	}

	p.flushVersion()
}

// classify routes one non-fenced line. lineOff is the byte offset of the
// line within the source, used to cut the preamble at the first version
// heading.
func (p *changelogParser) classify(line string, lineOff int) {
	trimmed := strings.TrimSuffix(line, "\r")

	if v, ok := parseChangelogVersion(trimmed); ok {
		if !p.started {
			// Clone rather than slice: a slice of p.src would pin the
			// whole document in memory for the lifetime of the returned
			// Changelog, and consumers cache these per repo.
			p.out.Preamble = strings.Clone(p.src[:lineOff])
			p.started = true
		}
		p.flushVersion()
		p.version = v
		p.versionOpen = true
		return
	}

	// Everything before the first version heading is preamble; nothing
	// else is captured from it.
	if !p.started {
		return
	}

	if title, ok := parseChangelogGroup(trimmed); ok {
		p.flushGroup()
		p.group = ChangelogGroup{Title: title}
		return
	}

	if body, ok := parseChangelogBullet(trimmed); ok {
		p.flushItem()
		p.item = append(p.item, body...)
		p.itemOpen = true
		return
	}

	p.appendContinuation(line)
}

// appendContinuation folds a line into the open item. Only indented
// lines continue an item (Decision 8: everything indented under a
// top-level bullet belongs to it); unindented prose closes the item and
// is discarded, so arbitrary text between bullets can never be mistaken
// for commit content. Blank lines are buffered rather than appended, so
// a trailing blank never lands in the item but a blank line between two
// indented blocks does not end it. Indentation is kept verbatim (see
// ChangelogGroup.Items), but a trailing carriage return is not: a CRLF
// document must yield the same item text as its LF twin.
func (p *changelogParser) appendContinuation(line string) {
	if !p.itemOpen {
		return
	}
	line = strings.TrimSuffix(line, "\r")
	if strings.TrimSpace(line) == "" {
		p.itemBlank++
		return
	}
	if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
		p.flushItem()
		return
	}
	for range p.itemBlank + 1 {
		p.item = append(p.item, '\n')
	}
	p.item = append(p.item, line...)
	p.itemBlank = 0
}

func (p *changelogParser) flushItem() {
	if !p.itemOpen {
		return
	}
	// string() copies, so the buffer's capacity is safe to reuse for the
	// next item.
	p.group.Items = append(p.group.Items, string(p.item))
	p.item = p.item[:0]
	p.itemOpen = false
	p.itemBlank = 0
}

func (p *changelogParser) flushGroup() {
	p.flushItem()
	if p.group.Title == "" && len(p.group.Items) == 0 {
		return
	}
	p.version.Groups = append(p.version.Groups, p.group)
	p.group = ChangelogGroup{}
}

func (p *changelogParser) flushVersion() {
	p.flushGroup()
	if p.versionOpen {
		p.out.Versions = append(p.out.Versions, p.version)
	}
	// Reset unconditionally: flushGroup above may have appended to
	// p.version.Groups, and on the no-open-version path that group would
	// otherwise leak into whichever version opens next.
	p.version = ChangelogVersion{}
	p.versionOpen = false
}

// parseChangelogVersion recognizes a version heading and normalizes its
// version string and date.
func parseChangelogVersion(line string) (ChangelogVersion, bool) {
	m := changelogVersionRE.FindStringSubmatch(line)
	if m == nil {
		return ChangelogVersion{}, false
	}

	inner := strings.TrimSpace(m[1])
	if strings.EqualFold(inner, "unreleased") {
		// Date is dropped for an unreleased section: git-cliff never
		// emits one, and the empty-for-unreleased shape is what
		// consumers pin.
		return ChangelogVersion{Version: "unreleased", Unreleased: true}, true
	}

	// Clone both: a regexp submatch is a subslice of the source, and
	// trimming preserves that aliasing, so an un-cloned 5-byte Version
	// would pin the entire document. See ParseChangelog's copy-out.
	return ChangelogVersion{
		Version: strings.Clone(trimVersionPrefix(inner)),
		Date:    strings.Clone(trimDateSeparator(m[2])),
	}, true
}

// trimVersionPrefix removes a single leading "v"/"V" when a digit
// follows, so "v0.4.2" becomes "0.4.2" while a word like "vnext" is left
// alone.
func trimVersionPrefix(s string) string {
	if len(s) < 2 || (s[0] != 'v' && s[0] != 'V') {
		return s
	}
	if s[1] < '0' || s[1] > '9' {
		return s
	}
	return s[1:]
}

// trimDateSeparator strips the single separator dash between the version
// bracket and the date, keeping whatever text follows verbatim. Only one
// leading dash rune is removed, so an em-dash separator works and a
// "---" prefix is not silently eaten.
func trimDateSeparator(trailer string) string {
	s := strings.TrimSpace(strings.TrimSuffix(trailer, "\r"))
	for _, dash := range []string{"-", "–", "—"} {
		if rest, ok := strings.CutPrefix(s, dash); ok {
			return strings.TrimSpace(rest)
		}
	}
	return s
}

// parseChangelogGroup recognizes a group heading and returns its title.
func parseChangelogGroup(line string) (string, bool) {
	m := changelogGroupRE.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	// Cloned for the same reason as Version/Date: the submatch aliases the
	// source document.
	return strings.Clone(strings.TrimSpace(m[1])), true
}

// parseChangelogBullet recognizes a top-level bullet and returns the item
// body with the marker and its trailing whitespace removed.
func parseChangelogBullet(line string) (string, bool) {
	loc := changelogBulletRE.FindStringIndex(line)
	if loc == nil {
		return "", false
	}
	return strings.TrimRight(line[loc[1]:], " \t"), true
}

// isChangelogFence reports whether a line opens or closes a fenced code
// block. It mirrors the module-wide rule (docparse's isFenceToggle):
// only a trimmed line starting with ``` toggles; ~~~ does not. The rule
// is duplicated rather than shared so neither package's frozen behavior
// is coupled to the other's.
func isChangelogFence(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "```")
}

// changelogLine returns the line of s starting at off (without its
// trailing newline) and the offset where the next line begins.
func changelogLine(s string, off int) (line string, next int) {
	rel := strings.IndexByte(s[off:], '\n')
	if rel < 0 {
		return s[off:], len(s)
	}
	return s[off : off+rel], off + rel + 1
}
