# Investigations

Time-boxed research spikes and validation experiments. Use an investigation
doc to answer a specific question before committing to a design or
implementation — e.g. proving a library can handle a requirement, reproducing
a bug, or validating a performance assumption.

Design docs, plans, and implementation docs can reference investigations by ID
(e.g. `INV-0001`) to document how open questions were resolved.

<!-- BEGIN DOCZ AUTO-GENERATED -->
## All Investigations

| ID | Title | Status | Date | Author | Link |
|----|-------|--------|------|--------|------|
| INV-0001 | Wiki Init Template and Init Enabled Fix | Concluded | 2026-04-02 | Donald Gifford | [0001-wiki-init-template-and-init-enabled-fix.md](0001-wiki-init-template-and-init-enabled-fix.md) |
| INV-0002 | Architectural Review and Cleanup Opportunities | Concluded | 2026-05-15 | Donald Gifford | [0002-architectural-review-and-cleanup-opportunities.md](0002-architectural-review-and-cleanup-opportunities.md) |
| INV-0003 | Init and Update Should Respect Config-Listed Types Only | Concluded | 2026-05-20 | Donald Gifford | [0003-init-and-update-should-respect-config-listed-types-only.md](0003-init-and-update-should-respect-config-listed-types-only.md) |
| INV-0004 | v1 Release Plan: TUI, Markdown Preview, and CLI Parity | Open | 2026-05-22 | Donald Gifford | [0004-v1-release-plan-tui-markdown-preview-and-cli-parity.md](0004-v1-release-plan-tui-markdown-preview-and-cli-parity.md) |
| INV-0005 | docz-api and docz-site: centralized cross-repo docz registry and viewer | Concluded | 2026-06-23 | Donald Gifford | [0005-docz-api-and-docz-site-centralized-cross-repo-docz-registry-and.md](0005-docz-api-and-docz-site-centralized-cross-repo-docz-registry-and.md) |
| INV-0006 | Per-package core requirements: docz CLI vs docz-api, docz-site, sdk-booty-sh | Open | 2026-07-03 | Donald Gifford | [0006-per-package-core-requirements-docz-cli-vs-docz-api-docz-site.md](0006-per-package-core-requirements-docz-cli-vs-docz-api-docz-site.md) |
| INV-0007 | docz internals required for the api additional_docs block | Concluded | 2026-08-10 | Donald Gifford | [0007-docz-internals-required-for-the-api-additionaldocs-block.md](0007-docz-internals-required-for-the-api-additionaldocs-block.md) |
| INV-0008 | Last Updated frontmatter field: scope, git semantics, and effort | Concluded | 2026-09-12 | Donald Gifford | [0008-last-updated-frontmatter-field-scope-git-semantics-and-effort.md](0008-last-updated-frontmatter-field-scope-git-semantics-and-effort.md) |
| INV-0009 | ToC regeneration in docz update and markdownlint MD051 | Concluded | 2026-09-13 | Donald Gifford | [0009-toc-regeneration-in-docz-update-and-markdownlint-md051.md](0009-toc-regeneration-in-docz-update-and-markdownlint-md051.md) |
| INV-0010 | IMPL plan parse and write-back API for doczcore (issue 100) | Concluded | 2026-09-13 | Donald Gifford | [0010-impl-plan-parse-and-write-back-api-for-doczcore-issue-100.md](0010-impl-plan-parse-and-write-back-api-for-doczcore-issue-100.md) |
| INV-0011 | Consolidating docz-api and docz-site into one repo: layout, module topology, and the v2 upgrade | Concluded | 2026-09-21 | Donald Gifford | [0011-consolidating-docz-api-and-docz-site-into-one-repo-layout.md](0011-consolidating-docz-api-and-docz-site-into-one-repo-layout.md) |
| INV-0012 | docz-site consumes the OpenAPI contract from the same repository | Concluded | 2026-09-23 | Donald Gifford | [0012-docz-site-consumes-the-openapi-contract-from-the-same-repository.md](0012-docz-site-consumes-the-openapi-contract-from-the-same-repository.md) |
| INV-0013 | docz-site deferred features after the move: link graph, lifecycle, labels | Open | 2026-09-23 | Donald Gifford | [0013-docz-site-deferred-features-after-the-move-link-graph-lifecycle.md](0013-docz-site-deferred-features-after-the-move-link-graph-lifecycle.md) |
| INV-0014 | Consolidate the docz-api and docz-site Helm charts into one chart | Concluded | 2026-09-25 | Donald Gifford | [0014-consolidate-the-docz-api-and-docz-site-helm-charts-into-one.md](0014-consolidate-the-docz-api-and-docz-site-helm-charts-into-one.md) |
| INV-0015 | v2.0.0 acceptance: swap a running 1.x environment to the latest beta | Open | 2026-10-02 | Donald Gifford | [0015-v200-acceptance-swap-a-running-1x-environment-to-the-latest-beta.md](0015-v200-acceptance-swap-a-running-1x-environment-to-the-latest-beta.md) |
| INV-0016 | One ADR per built-in document type | Open | 2026-10-02 | Donald Gifford | [0016-one-adr-per-built-in-document-type.md](0016-one-adr-per-built-in-document-type.md) |
| INV-0017 | docz-api: serve runbook metadata as structured fields | Open | 2026-10-02 | Donald Gifford | [0017-docz-api-serve-runbook-metadata-as-structured-fields.md](0017-docz-api-serve-runbook-metadata-as-structured-fields.md) |
| INV-0018 | docz-site: a step-aware runbook view | Open | 2026-10-02 | Donald Gifford | [0018-docz-site-a-step-aware-runbook-view.md](0018-docz-site-a-step-aware-runbook-view.md) |
| INV-0019 | docz-api sync to external documentation services: Confluence and Notion | Concluded | 2026-10-02 | Donald Gifford | [0019-docz-api-sync-to-external-documentation-services-confluence-and.md](0019-docz-api-sync-to-external-documentation-services-confluence-and.md) |
| INV-0020 | docz-api Confluence export: running confluence.Export without a checkout | Open | 2026-10-06 | Donald Gifford | [0020-docz-api-confluence-export-running-confluenceexport-without-a.md](0020-docz-api-confluence-export-running-confluenceexport-without-a.md) |
<!-- END DOCZ AUTO-GENERATED -->
<!-- BEGIN DOCZ AUTO-GENERATED -->
<!-- END DOCZ AUTO-GENERATED -->
