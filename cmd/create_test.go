package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// decodeOne decodes stdout as exactly one JSON object and fails on trailing
// output, which is the envelope's whole promise.
func decodeOne(t *testing.T, out *bytes.Buffer) createOutput {
	t.Helper()

	dec := json.NewDecoder(bytes.NewReader(out.Bytes()))

	var got createOutput
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out.String())
	}

	if dec.More() {
		t.Fatalf("stdout carries more than one JSON value:\n%s", out.String())
	}

	return got
}

func TestCreateJSON_EnvelopeCarriesUpdateAndNav(t *testing.T) {
	r, out, root := fixtureRunner(t, "design")

	r.Cfg.Wiki.AutoUpdate = true
	r.Cfg.Index.AutoUpdate = true

	if err := r.WikiInit(t.Context(), wikiInitOpts{siteName: "fixture"}); err != nil {
		t.Fatalf("WikiInit() = %v", err)
	}

	out.Reset()

	if err := r.Create(t.Context(), createOpts{format: formatJSON}, []string{"design", "A new design"}); err != nil {
		t.Fatalf("Create() = %v", err)
	}

	got := decodeOne(t, out)

	want := filepath.ToSlash(filepath.Join("docs", "design", "0003-a-new-design.md"))
	if got.SchemaVersion != 1 || got.ID != "DESIGN-0003" || got.Type != "design" ||
		got.Path != want || got.Title != "A new design" || got.GenericTemplate {
		t.Errorf("Create() envelope = %+v", got)
	}

	if _, err := os.Stat(filepath.Join(root, want)); err != nil {
		t.Errorf("the envelope's path does not exist: %v", err)
	}

	if got.Update == nil || got.Update.Type != "design" || got.Update.Docs != 3 || got.Update.Index.Action != "updated" {
		t.Errorf("Create().Update = %+v", got.Update)
	}

	if got.Nav == nil || got.Nav.Pages == 0 || !got.Nav.Written {
		t.Errorf("Create().Nav = %+v", got.Nav)
	}
}

func TestCreateJSON_NoUpdateIsNullFields(t *testing.T) {
	r, out, _ := fixtureRunner(t, "design")

	if err := r.Create(t.Context(), createOpts{format: formatJSON, noUpdate: true},
		[]string{"design", "Quiet"}); err != nil {
		t.Fatalf("Create() = %v", err)
	}

	if got := decodeOne(t, out); got.Update != nil || got.Nav != nil {
		t.Errorf("Create(--no-update) = update %+v nav %+v, want both null", got.Update, got.Nav)
	}

	if !strings.Contains(out.String(), `"update": null`) || !strings.Contains(out.String(), `"nav": null`) {
		t.Errorf("null fields are not spelled null:\n%s", out.String())
	}
}

func TestCreateJSON_GenericNoteStaysOnStderr(t *testing.T) {
	_, r, out, errOut := setupGenericType(t)

	if err := r.Create(t.Context(), createOpts{format: formatJSON}, []string{"frameworks", "A framework"}); err != nil {
		t.Fatalf("Create(frameworks) = %v", err)
	}

	if got := decodeOne(t, out); !got.GenericTemplate || got.ID != "FW-0001" {
		t.Errorf("Create(frameworks) = %+v, want generic_template true", got)
	}

	if !strings.Contains(errOut.String(), genericNote) {
		t.Errorf("stderr lacks the generic-template note:\n%s", errOut.String())
	}
}

func TestCreateJSON_FailurePrintsNoJSON(t *testing.T) {
	r, out, _ := fixtureRunner(t, "design")

	tc := r.Cfg.Types["design"]
	tc.Enabled = false
	r.Cfg.Types["design"] = tc

	err := r.Create(t.Context(), createOpts{format: formatJSON}, []string{"design", "Nope"})
	if err == nil {
		t.Fatal("Create() on a disabled type succeeded, want an error")
	}

	if out.Len() != 0 {
		t.Errorf("a failed creation printed:\n%s", out.String())
	}
}

func TestCreateJSON_BadFormat(t *testing.T) {
	r, _, _ := fixtureRunner(t, "design")

	err := r.Create(t.Context(), createOpts{format: "yaml"}, []string{"design", "Nope"})
	if err == nil || exitCodeFor(err) != 2 {
		t.Errorf("Create(--format yaml) = %v, want exit 2", err)
	}
}
