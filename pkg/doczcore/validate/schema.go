package validate

import (
	"sort"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// SchemaRegion is one region a schema requires: a kind, and the kind it
// must sit under.
type SchemaRegion struct {
	// Kind is the region kind.
	Kind string

	// Parent is the kind it nests under, or "" at the top level. A region
	// present at the wrong depth is region.wrong-parent, not a match: an
	// IMPL's tasks region outside a phase belongs to no phase, and reading
	// it as one would attribute its tasks to whatever came before.
	Parent string
}

// Schema is the set of regions a document must carry.
//
// It is read from a marker skeleton by SchemaFromMarkers, so there is no
// schema language and nothing a schema can require that a document cannot
// show (DESIGN-0015 §3). Every kind listed is required at least once under
// the same parent. A kind not listed is optional, and when present it is
// still checked by its content rule. A schema therefore only tightens by
// growing.
//
// The zero Schema requires nothing, which makes Document a well-formedness
// check: that is what a document whose schema name resolves nowhere gets,
// and it is deliberately still useful.
type Schema struct {
	Regions []SchemaRegion
}

// SchemaFromMarkers reads a schema from a marker skeleton — or from any
// marked document, which is what makes the derivation test possible: a
// template and its skeleton must yield the same Schema, so the pair can
// only be edited together.
//
// The legacy ToC pair counts, reported as the "toc" kind like everywhere
// else, so a skeleton requires a ToC by carrying the pair it always had.
func SchemaFromMarkers(skeleton []byte) Schema {
	regions := docparse.Regions(skeleton)

	seen := make(map[SchemaRegion]bool, len(regions))

	var out []SchemaRegion

	for i, r := range regions {
		entry := SchemaRegion{Kind: r.Kind, Parent: parentKind(regions, i)}
		if seen[entry] {
			continue
		}

		seen[entry] = true
		out = append(out, entry)
	}

	// Sorted so two skeletons that require the same regions compare equal
	// whatever order they list them in. A schema is a set, not a sequence:
	// it says which regions must exist, never where.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Parent != out[j].Parent {
			return out[i].Parent < out[j].Parent
		}

		return out[i].Kind < out[j].Kind
	})

	return Schema{Regions: out}
}

// parentKind returns the kind of the innermost region enclosing the one at
// index i, or "" when it is top-level.
func parentKind(regions []docparse.Region, i int) string {
	r := regions[i]
	if r.Depth == 0 {
		return ""
	}

	for j := i - 1; j >= 0; j-- {
		if regions[j].Depth == r.Depth-1 && regions[j].Start < r.Start && regions[j].End >= r.End {
			return regions[j].Kind
		}
	}

	return ""
}
