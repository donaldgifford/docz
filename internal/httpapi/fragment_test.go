package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/internal/authorize"
	"github.com/donaldgifford/docz/v2/internal/store"
)

// fragmentDoc is a document with frontmatter and markers, linking to a
// sibling document, a published page, a file nothing publishes, and an
// external URL.
const fragmentDoc = `---
id: FW-0002
title: Rendered
status: Draft
author: Jane
created: 2026-03-04
---

# FW-0002: Rendered

<!--docz:overview:start-->
## Overview

See [the intro](0001-intro.md#why), [setup](../guides/setup.md),
[the script](../../scripts/run.sh), and [the site](https://example.com).

<!--docz:overview:end-->
`

func fragmentStore() *fakeStore {
	st := seededStore()
	st.docs[1] = append(st.docs[1], store.Document{
		ID: 102, RepoID: 1, Type: "frameworks", DocID: "FW-0002", Title: "Rendered",
		Status: validText("Draft"), Path: "docs/frameworks/0002-rendered.md",
		GitSha: "ghi", ContentHash: "hash3", RawMd: fragmentDoc,
	})
	st.repos[0].ConfigSnapshot = json.RawMessage(`{"docs_dir":"docs","html":{"theme":"charcoal"}}`)

	return st
}

func TestGetDocHTML(t *testing.T) {
	t.Parallel()

	srv := testServer(fragmentStore(), fixedAuthorizer{allowed: authorize.AllowedRepos{1, 2}})
	rec := doGet(t, srv, "/api/v1/repos/acme/platform/types/FW/docs/FW-0002/html")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET html = %d %s", rec.Code, rec.Body)
	}

	var body DocumentHTMLDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body.DocID != "FW-0002" || !strings.HasPrefix(body.Hash, "sha256:") || body.Inferred {
		t.Errorf("body = %+v", body)
	}

	for _, want := range []string{
		`<div class="docz-page" data-theme="charcoal" data-doc="FW-0002">`,
		`<p class="eyebrow">platform · frameworks</p>`,
		`href="/acme/platform/frameworks/FW-0001#why"`,
		`href="/acme/platform/pages/guides/setup.md"`,
		`href="https://example.com"`,
	} {
		if !strings.Contains(body.HTML, want) {
			t.Errorf("the fragment lacks %s", want)
		}
	}

	if strings.Contains(body.HTML, "run.sh\"") || strings.Contains(body.HTML, "<style") {
		t.Errorf("the fragment links an unpublished file or carries a style:\n%s", body.HTML)
	}
}

func TestGetDocHTMLNotFound(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		allowed authorize.AllowedRepos
		path    string
	}{
		"unknown document": {authorize.AllowedRepos{1}, "/api/v1/repos/acme/platform/types/FW/docs/FW-0099/html"},
		"unknown type":     {authorize.AllowedRepos{1}, "/api/v1/repos/acme/platform/types/nope/docs/FW-0002/html"},
		"repo not allowed": {authorize.AllowedRepos{999}, "/api/v1/repos/acme/platform/types/FW/docs/FW-0002/html"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			srv := testServer(fragmentStore(), fixedAuthorizer{allowed: tc.allowed})
			if rec := doGet(t, srv, tc.path); rec.Code != http.StatusNotFound {
				t.Errorf("GET %s = %d, want 404", tc.path, rec.Code)
			}
		})
	}
}

func TestSnapshotTheme(t *testing.T) {
	t.Parallel()

	for snapshot, want := range map[string]string{
		`{"html":{"theme":"classic"}}`: "classic",
		`{"html":{"theme":"neon"}}`:    "",
		`{"docs_dir":"docs"}`:          "",
		`not json`:                     "",
	} {
		if got := snapshotTheme(json.RawMessage(snapshot)); got != want {
			t.Errorf("snapshotTheme(%s) = %q, want %q", snapshot, got, want)
		}
	}
}

func TestSiteResolver(t *testing.T) {
	t.Parallel()

	repo := &store.Repo{Owner: "acme", Name: "platform", DocsDir: "docs"}
	types := []store.DocType{{Name: "frameworks", Dir: "frameworks", IDPrefix: "FW"}, {Name: "rfc", Dir: "rfc", IDPrefix: "RFC"}}
	pages := []store.ListRepoPagesRow{{Path: "guides", RepoPath: "docs/guides/README.md"}}
	resolve := siteResolver(repo, types, pages)
	from := "docs/frameworks/0002-rendered.md"

	for href, want := range map[string]string{
		"0001-intro.md":            "/acme/platform/frameworks/FW-0001",
		"../rfc/0012-x.md#risks":   "/acme/platform/rfc/RFC-0012#risks",
		"../guides/README.md":      "/acme/platform/pages/guides",
		"../frameworks/README.md":  "",
		"../../../etc/passwd":      "",
		"/docs/rfc/0012-x.md":      "",
		"mailto:a@example.com":     "",
		"../other/0003-unknown.md": "",
		"../../docs/rfc/0004-y.md": "/acme/platform/rfc/RFC-0004",
		"#overview":                "",
	} {
		if got := resolve(from, href); got != want {
			t.Errorf("resolve(%q) = %q, want %q", href, got, want)
		}
	}
}
