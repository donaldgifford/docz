package doczcore_test

import (
	"os/exec"
	"strings"
	"testing"
)

// modulePath is the v2 module path. Spelled out rather than read from go.mod
// so a test that is about import paths does not depend on parsing one.
const modulePath = "github.com/donaldgifford/docz/v2"

// corePackages are the type-agnostic packages of the docz API. Listed rather
// than globbed so adding a core package is a deliberate edit to this list,
// which is where a reviewer will look to ask whether it belongs.
var corePackages = []string{
	"config", "document", "docparse", "docwrite", "toc", "kinds", "validate",
}

// listPackages returns every package under pkg/.
//
// Addressed by module path rather than by the ./pkg/... pattern: a test runs
// with its own package directory as the working directory, where a relative
// pattern resolves to nothing.
func listPackages(t *testing.T) []string {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "go", "list", modulePath+"/pkg/...").Output()
	if err != nil {
		t.Fatalf("go list %s/pkg/...: %v", modulePath, err)
	}

	return strings.Split(strings.TrimSpace(string(out)), "\n")
}

// deps returns the full transitive import list of a package, production
// imports only.
//
// Test imports are excluded, and that is the point rather than a shortcut: a
// core package's test may import internal/template to pin its behaviour
// against the embedded templates, and several do. What must not happen is a
// core package depending on a type package in the code a consumer compiles.
func deps(t *testing.T, pkg string) []string {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", pkg).Output()
	if err != nil {
		t.Fatalf("go list -deps %s: %v", pkg, err)
	}

	return strings.Split(strings.TrimSpace(string(out)), "\n")
}

// TestLayerRules enforces R2: pkg/doczcore is type-agnostic, so no core
// package may import a type package (ADR-0002 R2, DESIGN-0014 §1).
//
// The rule is what makes a grammar a contract over region kinds rather than
// over document types (R7). Break it and a repo's custom type stops getting
// the same readers a built-in does, because the core would know which types
// exist. DESIGN-0014 §6 chose a test over a depguard rule, and this is it.
func TestLayerRules_CoreNeverImportsATypePackage(t *testing.T) {
	t.Parallel()

	// Every package directly under pkg/ that is not doczcore is a type
	// package or a policy package above the core. None may appear in a core
	// package's dependency list.
	forbidden := []string{
		modulePath + "/pkg/impl",
		modulePath + "/pkg/rfc",
		modulePath + "/pkg/adr",
		modulePath + "/pkg/design",
		modulePath + "/pkg/investigation",
		modulePath + "/pkg/repo",
		modulePath + "/pkg/wiki",
	}

	for _, name := range corePackages {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pkg := modulePath + "/pkg/doczcore/" + name

			for _, dep := range deps(t, pkg) {
				for _, bad := range forbidden {
					if dep == bad {
						t.Errorf("%s imports %s; the core is type-agnostic (R2)", pkg, bad)
					}
				}

				// cmd/ is the CLI shell. A core package reaching into it
				// would make the library depend on its own first consumer.
				if strings.HasPrefix(dep, modulePath+"/cmd") {
					t.Errorf("%s imports %s; the core does not depend on the CLI", pkg, dep)
				}
			}
		})
	}
}

// TestLayerRules_NoTelemetryUnderPkg enforces that the public surface stays
// stdlib plus yaml.v3 (ADR-0001 Neutral, ADR-0002 Decision 5).
//
// The library is traceable, not tracing: it takes a context at L3 and reads
// hooks from it, and it never imports a telemetry module, logs, or prints.
// This is not a style preference. A library that picks a tracing SDK picks it
// for every consumer, and docz-api already has its own.
func TestLayerRules_NoTelemetryUnderPkg(t *testing.T) {
	t.Parallel()

	banned := []string{
		"go.opentelemetry.io",
		"github.com/prometheus/",
		"go.uber.org/zap",
		"github.com/sirupsen/logrus",
		"github.com/rs/zerolog",
		"github.com/spf13/cobra",
		"github.com/spf13/viper",
	}

	packages := listPackages(t)
	if len(packages) < len(corePackages) {
		t.Fatalf("go list found %d packages under pkg/, want at least %d",
			len(packages), len(corePackages))
	}

	for _, pkg := range packages {
		t.Run(strings.TrimPrefix(pkg, modulePath+"/"), func(t *testing.T) {
			t.Parallel()

			for _, dep := range deps(t, pkg) {
				for _, bad := range banned {
					if strings.HasPrefix(dep, bad) {
						t.Errorf("%s depends on %s; the public core is stdlib plus yaml.v3", pkg, dep)
					}
				}
			}
		})
	}
}

// The core's own dependency list is short by design, and saying so out loud
// is what turns "stdlib plus yaml.v3" from an intention into a check. A new
// third-party module under pkg/ fails here and has to be argued for.
func TestLayerRules_ThirdPartyDependencies(t *testing.T) {
	t.Parallel()

	allowed := map[string]bool{
		"go.yaml.in/yaml/v3": true,
	}

	for _, pkg := range listPackages(t) {
		for _, dep := range deps(t, pkg) {
			// A stdlib path has no dot in its first segment; anything in this
			// module is ours.
			first, _, _ := strings.Cut(dep, "/")
			if !strings.Contains(first, ".") || strings.HasPrefix(dep, modulePath) {
				continue
			}

			if allowed[dep] {
				continue
			}

			// Allow a subpackage of an allowed module.
			ok := false

			for module := range allowed {
				if strings.HasPrefix(dep, module+"/") {
					ok = true

					break
				}
			}

			if !ok {
				t.Errorf("%s depends on %s, which is not in the allow-list", pkg, dep)
			}
		}
	}
}

// TestLayerRules_TheDetectorWorks guards the two rules above from passing
// vacuously.
//
// A dependency walk that returned nothing, or a prefix match that never
// matched, would make both tests green forever while the rules rotted. cmd/
// really does depend on Cobra, so finding it there proves the mechanism sees
// what it claims to see — and not finding it under pkg/ then means something.
func TestLayerRules_TheDetectorWorks(t *testing.T) {
	t.Parallel()

	found := false

	for _, dep := range deps(t, modulePath+"/cmd") {
		if strings.HasPrefix(dep, "github.com/spf13/cobra") {
			found = true

			break
		}
	}

	if !found {
		t.Error("the dependency walk cannot see Cobra in cmd/, so the layer rules prove nothing")
	}
}
