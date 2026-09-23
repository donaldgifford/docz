package httpapi

import (
	"encoding/json"
	"strings"

	"github.com/donaldgifford/docz-api/internal/store"
)

// resolveType maps a URL {type} segment to its canonical type name by matching
// against each type's name, id_prefix (case-insensitive), or any alias
// (case-insensitive) — the same resolution DESIGN-0007 gives the CLI, so
// .../types/frameworks/docs and .../types/FW/docs are equivalent. It operates on
// the repo's already-loaded doc_types rows, so no live docz config is needed at
// serve time. Returns ("", false) when nothing matches.
func resolveType(types []store.DocType, input string) (canonical string, ok bool) {
	for i := range types {
		dt := &types[i]
		if dt.Name == input {
			return dt.Name, true
		}
		if strings.EqualFold(dt.IDPrefix, input) {
			return dt.Name, true
		}
		var aliases []string
		if err := json.Unmarshal(dt.Aliases, &aliases); err != nil {
			continue
		}
		for _, a := range aliases {
			if strings.EqualFold(a, input) {
				return dt.Name, true
			}
		}
	}
	return "", false
}
