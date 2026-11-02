---
name: docz:show
description: >
  Show the resolved template for a docz document type.
when_to_use: >
  User wants to understand the structure of a doc type before creating one.
  "show me the RFC template", "what sections does an IMPL doc have", "what
  does an ADR look like", "show me the investigation template", "what goes
  in a design doc".
argument-hint: "<type>"
---

# Docz Show

Print the resolved template for a document type with `docz template show <type>`.

## Process

1. **Resolve the type** from the user's description:

   | User says | CLI type |
   |-----------|----------|
   | ADR, architecture decision | `adr` |
   | RFC, proposal | `rfc` |
   | design doc, design spec | `design` |
   | implementation, impl | `impl` |
   | plan, planning doc | `plan` |
   | investigation, spike, INV | `inv` |

2. **Print the template**:
   ```bash
   docz template show <type>
   ```

   The CLI embeds every default template and resolves overrides automatically (config path → `docs/templates/<type>.md` → embedded default), so this reflects exactly what `docz create` would write. It works for disabled types too — template resolution ignores `enabled`.

   If `docz` is not on `PATH`, stop and tell the user to install it (≥ v1.2.0, e.g. `mise use github:donaldgifford/docz@latest`). This skill has no offline path.

3. **Explain the sections** in plain language after printing the raw template — what to write in each section and common mistakes to avoid.

4. **Offer next steps**:
   - Suggest `docz create <type> "<title>"` to create a document from this template
   - Mention `docz template override <type>` if the user wants to customize the template

## Reference

Read `../docz/references/doc-types.md` for detailed section-by-section guidance on each type.
