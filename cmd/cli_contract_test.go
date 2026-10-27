package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	clischema "github.com/donaldgifford/docz/v2/api/cli"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// contractDir holds one committed output per governed command, captured
// with -update and re-validated on every run, so a schema change that would
// reject output docz has already shipped fails here too.
var contractDir = filepath.Join("testdata", "cli-contract", "v1")

// contractSchemas compiles every embedded schema, closed as committed, with
// format assertions on.
func contractSchemas(t *testing.T) map[string]*jsonschema.Schema {
	t.Helper()

	c := jsonschema.NewCompiler()
	c.AssertFormat()

	for _, name := range clischema.Names {
		raw, err := clischema.Schema(name)
		if err != nil {
			t.Fatal(err)
		}

		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("schema %s: %v", name, err)
		}

		if err := c.AddResource(name+".schema.json", doc); err != nil {
			t.Fatalf("schema %s: %v", name, err)
		}
	}

	out := make(map[string]*jsonschema.Schema, len(clischema.Names))

	for _, name := range clischema.Names {
		sch, err := c.Compile(name + ".schema.json")
		if err != nil {
			t.Fatalf("compiling %s: %v", name, err)
		}

		out[name] = sch
	}

	return out
}

// contractCheck validates one output against its schema and, under -update,
// records it as the committed fixture.
func contractCheck(t *testing.T, sch *jsonschema.Schema, fixture string, output []byte) {
	t.Helper()

	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(output))
	if err != nil {
		t.Fatalf("%s: output is not JSON: %v\n%s", fixture, err, output)
	}

	if err := sch.Validate(inst); err != nil {
		t.Errorf("%s: output does not match its schema:\n%v\n%s", fixture, err, output)
	}

	if *updateExportGoldens {
		if err := os.MkdirAll(contractDir, config.DirMode); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(contractDir, fixture+".json"), output, config.FileMode); err != nil {
			t.Fatal(err)
		}
	}
}

// TestCLIJSONContract runs every governed command through a Runner over the
// parity fixtures and validates its output against the closed schema. An
// added, renamed, or removed field fails here (DESIGN-0024 §3).
func TestCLIJSONContract(t *testing.T) {
	schemas := contractSchemas(t)

	t.Run("inspect", func(t *testing.T) {
		for _, tc := range []struct{ fixture, arg, name string }{
			{"design", "DESIGN-0001", "inspect-design"},
			{"impl", "IMPL-0001", "inspect-impl"},
			{"custom", "FW-0001", "inspect-custom"},
		} {
			r, out, _ := fixtureRunner(t, tc.fixture)
			if err := r.inspect(t.Context(), formatJSON, tc.arg); err != nil {
				t.Fatalf("inspect(%s) = %v", tc.arg, err)
			}

			contractCheck(t, schemas["inspect"], tc.name, out.Bytes())
		}

		r, out, root := fixtureRunner(t, "design")
		rel := filepath.Join("docs", "design", "draft.md")
		vtestWrite(t, filepath.Join(root, rel), "## Overview\n\nText.\n")

		if err := r.inspect(t.Context(), formatJSON, rel); err != nil {
			t.Fatalf("inspect(%s) = %v", rel, err)
		}

		contractCheck(t, schemas["inspect"], "inspect-no-frontmatter", out.Bytes())
	})

	t.Run("validate", func(t *testing.T) {
		// Findings exit non-zero; the output is the subject, so the errors
		// are discarded.
		r, out, _ := fixtureRunner(t, "rfc")
		_ = r.validate(t.Context(), validateOpts{format: formatJSON}, nil)
		contractCheck(t, schemas["validate"], "validate-repository", out.Bytes())

		out.Reset()
		_ = r.validate(t.Context(), validateOpts{format: formatJSON}, []string{"RFC-0001"})
		contractCheck(t, schemas["validate"], "validate-document", out.Bytes())

		vr, vout, _ := vtestRunner(t, vtestUnmarked())
		_ = vr.validate(t.Context(), validateOpts{format: formatJSON, fix: true}, nil)
		contractCheck(t, schemas["validate"], "validate-fix", vout.Bytes())
	})

	t.Run("create", func(t *testing.T) {
		r, out, _ := fixtureRunner(t, "design")
		r.Cfg.Wiki.AutoUpdate = true

		if err := r.WikiInit(t.Context(), wikiInitOpts{siteName: "fixture"}); err != nil {
			t.Fatal(err)
		}

		out.Reset()

		if err := r.Create(t.Context(), createOpts{format: formatJSON}, []string{"design", "Contract"}); err != nil {
			t.Fatalf("create = %v", err)
		}

		contractCheck(t, schemas["create"], "create", out.Bytes())
	})

	t.Run("list", func(t *testing.T) {
		r, out, _ := fixtureRunner(t, "custom")
		if err := r.List(t.Context(), listOpts{format: formatJSON}, nil); err != nil {
			t.Fatalf("list = %v", err)
		}

		contractCheck(t, schemas["list"], "list", out.Bytes())
	})
}

// TestCLIJSONContractFixtures re-validates every committed output, including
// the review record fixtures docz-review's own tests share.
func TestCLIJSONContractFixtures(t *testing.T) {
	schemas := contractSchemas(t)

	entries, err := os.ReadDir(contractDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) == 0 {
		t.Fatal("no committed contract fixtures")
	}

	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		schema, _, _ := strings.Cut(name, "-")

		sch, ok := schemas[schema]
		if !ok {
			t.Errorf("%s names no schema", e.Name())

			continue
		}

		raw, err := os.ReadFile(filepath.Join(contractDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}

		inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}

		if err := sch.Validate(inst); err != nil {
			t.Errorf("%s no longer matches %s.schema.json:\n%v", e.Name(), schema, err)
		}
	}
}
