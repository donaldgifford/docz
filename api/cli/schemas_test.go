package cli

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

const idBase = "https://github.com/donaldgifford/docz/schemas/cli/"

var versions = map[string]int{
	"inspect":  SchemaVersionInspect,
	"validate": SchemaVersionValidate,
	"create":   SchemaVersionCreate,
	"review":   SchemaVersionReview,
}

func TestSchemas(t *testing.T) {
	t.Parallel()

	for _, name := range Names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			raw, err := Schema(name)
			if err != nil {
				t.Fatalf("Schema(%q) error = %v", name, err)
			}

			var doc map[string]any
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("Schema(%q) is not JSON: %v", name, err)
			}

			version, versioned := versions[name]
			if !versioned {
				version = 1
			}

			if got, want := doc["$id"], idBase+name+"/v"+strconv.Itoa(version); got != want {
				t.Errorf("Schema(%q) $id = %v, want %s", name, got, want)
			}

			if _, ok := doc["$version"].(string); !ok {
				t.Errorf("Schema(%q) has no string $version", name)
			}

			if versioned {
				if got := schemaVersionConst(doc); got != version {
					t.Errorf("Schema(%q) schema_version const = %d, want %d", name, got, version)
				}
			}

			walkObjects(t, name, "#", doc)
		})
	}
}

func TestSchemaUnknown(t *testing.T) {
	t.Parallel()

	if _, err := Schema("nope"); err == nil {
		t.Error(`Schema("nope") error = nil, want an error`)
	}
}

// walkObjects fails for any object schema that is not closed.
func walkObjects(t *testing.T, name, path string, node any) {
	t.Helper()

	switch v := node.(type) {
	case map[string]any:
		if isObjectSchema(v) {
			if ap, ok := v["additionalProperties"].(bool); !ok || ap {
				t.Errorf("Schema(%q) %s: object without additionalProperties: false", name, path)
			}
		}

		for k, child := range v {
			walkObjects(t, name, path+"/"+k, child)
		}
	case []any:
		for i, child := range v {
			walkObjects(t, name, path+"/"+strconv.Itoa(i), child)
		}
	}
}

func isObjectSchema(m map[string]any) bool {
	switch typ := m["type"].(type) {
	case string:
		return typ == "object"
	case []any:
		for _, x := range typ {
			if x == "object" {
				return true
			}
		}
	}

	return false
}

// schemaVersionConst finds the schema_version const at the top level or in
// every branch of a top-level oneOf, returning 0 on a disagreement.
func schemaVersionConst(doc map[string]any) int {
	if props, ok := doc["properties"].(map[string]any); ok {
		return constOf(props)
	}

	branches, _ := doc["oneOf"].([]any)
	defs, _ := doc["$defs"].(map[string]any)

	found := 0

	for _, b := range branches {
		ref, _ := b.(map[string]any)["$ref"].(string)
		def, _ := defs[strings.TrimPrefix(ref, "#/$defs/")].(map[string]any)
		props, _ := def["properties"].(map[string]any)

		v := constOf(props)
		if found != 0 && v != found {
			return 0
		}

		found = v
	}

	return found
}

func constOf(props map[string]any) int {
	sv, _ := props["schema_version"].(map[string]any)
	c, _ := sv["const"].(float64)

	return int(c)
}
