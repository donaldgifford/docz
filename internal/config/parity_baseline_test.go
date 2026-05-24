package config_test

import (
	"bytes"
	"reflect"
	"testing"
	"text/template"

	"go.yaml.in/yaml/v3"

	"github.com/donaldgifford/docz/internal/config"
	doctemplate "github.com/donaldgifford/docz/internal/template"
)

// TestDoczYAMLTemplate_RoundTripsToDefaultConfig is the IMPL-0006 Phase 1
// regression guard: the embedded `.docz.yaml.tmpl` rendered with
// DefaultConfig() and parsed back must equal DefaultConfig(). Catches
// drift between the template, DefaultConfig, and the YAML schema if
// any of the three change without the others.
func TestDoczYAMLTemplate_RoundTripsToDefaultConfig(t *testing.T) {
	tmplSrc, err := doctemplate.EmbeddedDoczYAML()
	if err != nil {
		t.Fatalf("loading template: %v", err)
	}

	tmpl, err := template.New("docz_yaml").Parse(tmplSrc)
	if err != nil {
		t.Fatalf("parsing template: %v", err)
	}

	want := config.DefaultConfig()
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, want); err != nil {
		t.Fatalf("rendering template: %v", err)
	}

	var got config.Config
	if err := yaml.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshalling rendered yaml: %v\nrendered:\n%s", err, buf.String())
	}

	if !reflect.DeepEqual(want, got) {
		t.Errorf("rendered template did not round-trip to DefaultConfig()\nwant: %#v\ngot:  %#v", want, got)
	}
}

// TestDoczYAMLTemplate_RetainsCommentHeader guards the human-readable
// header block at the top of the rendered file. Pure yaml.Marshal would
// drop comments; the template approach preserves them. If a future change
// switches back to marshal, this catches it.
func TestDoczYAMLTemplate_RetainsCommentHeader(t *testing.T) {
	tmplSrc, err := doctemplate.EmbeddedDoczYAML()
	if err != nil {
		t.Fatalf("loading template: %v", err)
	}

	tmpl, err := template.New("docz_yaml").Parse(tmplSrc)
	if err != nil {
		t.Fatalf("parsing template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config.DefaultConfig()); err != nil {
		t.Fatalf("rendering template: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"# .docz.yaml -- configuration for the docz CLI",
		"# About the `types:` block",
		"# Documentation: https://github.com/donaldgifford/docz",
	} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("rendered output missing header line %q", want)
		}
	}
}
