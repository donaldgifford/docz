package cmd

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// ADR-0003 removes `plan` from the built-in catalogue on the v2 line. A repo
// that kept its `types.plan` block does not lose its documents: the block now
// declares a **custom** type, and everything except creating a new one keeps
// working.
//
// These tests are the promise. Without them the fallback is an argument in a
// design document rather than something the binary does.

// legacyPlanConfig is a v1-era .docz.yaml carrying the block. Two types, so a
// test can tell "the custom type worked" from "nothing worked".
const legacyPlanConfig = `docs_dir: docs
types:
  rfc:
    enabled: true
    dir: rfc
    id_prefix: RFC
    id_width: 4
    statuses: [Draft, Accepted]
    status_field: status
  plan:
    enabled: true
    dir: plan
    id_prefix: PLAN
    id_width: 4
    statuses: [Draft, In Progress, Completed]
    status_field: status
`

// legacyPlanDoc is a PLAN document such a repo already has on disk. Written by
// hand because there is no plan template left to create one from, which is the
// situation the fallback exists for.
const legacyPlanDoc = `---
id: PLAN-0001
title: "A plan written under v1"
status: In Progress
author: A Maintainer
created: 2026-01-15
---

# PLAN-0001: A plan written under v1

## Goal

Ship the thing.
`

// setupLegacyPlan writes the legacy config into a fresh temp dir and points the
// repo root at it, the same way setupINV0003Test does.
func setupLegacyPlan(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, ".docz.yaml"), []byte(legacyPlanConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgFile = ""
	repoRoot = dir
	docsDir = ""

	t.Cleanup(func() {
		cfgFile = ""
		repoRoot = ""
		docsDir = ""
		appCfg = config.DefaultConfig()
		runner = nil
	})

	return dir
}

// runRootErr is runRoot for a command expected to fail: it returns the error
// and the output rather than ending the test, since the message is the thing
// under test.
func runRootErr(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var buf bytes.Buffer

	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(args)

	err := rootCmd.Execute()

	return buf.String(), err
}

// TestLegacyPlan_LoadsAsACustomType is the base case: the block still parses,
// and the type resolves by name, by id prefix, and case-insensitively, exactly
// as any other custom type does.
func TestLegacyPlan_LoadsAsACustomType(t *testing.T) {
	dir := setupLegacyPlan(t)

	cfg, err := config.Load(filepath.Join(dir, ".docz.yaml"), dir)
	if err != nil {
		t.Fatalf("Load = %v, want nil", err)
	}

	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("Validate = %v, want nil: a dormant or live plan block must keep loading", err)
	}

	// It is no longer a built-in, which is the whole of the change.
	if _, ok := config.LookupDocType("plan"); ok {
		t.Error("LookupDocType(plan) = true; ADR-0003 removed the built-in")
	}

	for _, token := range []string{"plan", "PLAN", "Plan"} {
		got, err := cfg.ValidateType(token)
		if err != nil {
			t.Errorf("ValidateType(%q) = %v, want it to resolve as a custom type", token, err)

			continue
		}

		if got != "plan" {
			t.Errorf("ValidateType(%q) = %q, want plan", token, got)
		}
	}

	// And it is in the enabled set, so no-argument commands reach it. Custom
	// types sort after the built-ins, which is the one visible difference.
	enabled := cfg.EnabledTypes()
	if enabled[len(enabled)-1] != "plan" {
		t.Errorf("EnabledTypes() = %v, want plan appended as a custom type", enabled)
	}
}

// TestLegacyPlan_InitListAndUpdateWork covers the three commands a repo runs
// most. All three have to keep working over docs/plan, because a repo that
// upgraded docz did not ask to lose its index.
func TestLegacyPlan_InitListAndUpdateWork(t *testing.T) {
	dir := setupLegacyPlan(t)

	runRoot(t, "init")

	planDir := filepath.Join(dir, "docs", "plan")
	if _, err := os.Stat(planDir); err != nil {
		t.Fatalf("docs/plan was not scaffolded: %v", err)
	}

	readme := filepath.Join(planDir, config.IndexFileName)
	if _, err := os.Stat(readme); err != nil {
		t.Fatalf("docs/plan/README.md was not created: %v", err)
	}

	if err := os.WriteFile(filepath.Join(planDir, "0001-a-plan-written-under-v1.md"),
		[]byte(legacyPlanDoc), 0o644); err != nil {
		t.Fatal(err)
	}

	runRoot(t, "update")

	body, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(body), "PLAN-0001") {
		t.Errorf("docs/plan/README.md does not index the document:\n%s", body)
	}

	// list goes through the Runner's own writer rather than Cobra's, so it is
	// driven directly over a buffer with the config the repo actually has.
	cfg, err := config.Load(filepath.Join(dir, ".docz.yaml"), dir)
	if err != nil {
		t.Fatalf("Load = %v, want nil", err)
	}

	// Absolute, matching installListRunner: the list handler joins DocsDir
	// itself rather than going through Runner.inRepo.
	cfg.DocsDir = filepath.Join(dir, "docs")

	listStatus = ""
	listFormat = "table"

	for _, args := range [][]string{nil, {"plan"}, {"PLAN"}} {
		var out bytes.Buffer

		runner = &Runner{
			Cfg:      cfg,
			Out:      &out,
			Err:      io.Discard,
			Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
			Now:      time.Now,
			Git:      staticGit{},
			RepoRoot: dir,
		}

		if err := runList(nil, args); err != nil {
			t.Fatalf("list %v = %v, want nil", args, err)
		}

		if !strings.Contains(out.String(), "PLAN-0001") {
			t.Errorf("list %v does not show the plan document:\n%s", args, out.String())
		}
	}

	runner = nil
}

// TestLegacyPlan_CreateFailsNamingTheFix is the one command that stops working,
// and the message is the whole of the user experience: it has to name the file
// to create, not just say no.
func TestLegacyPlan_CreateFailsNamingTheFix(t *testing.T) {
	dir := setupLegacyPlan(t)

	runRoot(t, "init")

	out, err := runRootErr(t, "create", "plan", "Something new")
	if err == nil {
		t.Fatalf("create plan succeeded; there is no plan template left (output: %s)", out)
	}

	want := filepath.Join(dir, "docs", "templates", "plan.md")
	if !strings.Contains(err.Error(), want) {
		t.Errorf("create plan error does not name %s as the fix:\n%v", want, err)
	}
}

// TestLegacyPlan_TemplateOverrideNamesTheSamePath pins where the fallback
// stands today.
//
// repo.ExportTemplate scaffolds the generic template-and-schema pair for a type
// whose template resolves nowhere (IMPL-0018 Phase 3), but cmd/template.go does
// not go through it until the Phase 5 swap. Until then the command fails and
// names the path a person would create, which is the same path `create` names.
// This test is what will fail when the swap lands, and that failure is the
// reminder to assert the scaffold instead.
func TestLegacyPlan_TemplateOverrideNamesTheSamePath(t *testing.T) {
	dir := setupLegacyPlan(t)

	runRoot(t, "init")

	out, err := runRootErr(t, "template", "override", "plan")
	if err == nil {
		t.Fatalf("template override plan succeeded; Phase 5 has landed, "+
			"so assert the scaffolded pair here instead (output: %s)", out)
	}

	want := filepath.Join(dir, "docs", "templates", "plan.md")
	if !strings.Contains(err.Error(), want) {
		t.Errorf("template override plan error does not name %s:\n%v", want, err)
	}
}
