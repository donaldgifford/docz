---
id: INV-0006
title: "cosign v3 cannot verify our SLSA provenance attestations"
status: Concluded
author: Donald Gifford
created: 2026-08-10
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0006: cosign v3 cannot verify our SLSA provenance attestations

**Status:** Concluded
**Author:** Donald Gifford
**Date:** 2026-08-10

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [F1 — The documented command fails on every published version](#f1--the-documented-command-fails-on-every-published-version)
  - [F2 — The provenance exists, is correct, and is signed by the right identity](#f2--the-provenance-exists-is-correct-and-is-signed-by-the-right-identity)
  - [F3 — Two attachment schemes coexist on the same digest](#f3--two-attachment-schemes-coexist-on-the-same-digest)
  - [F4 — cosign v2 verifies the same artifact with the same command](#f4--cosign-v2-verifies-the-same-artifact-with-the-same-command)
  - [F5 — GHCR does not serve the OCI referrers API](#f5--ghcr-does-not-serve-the-oci-referrers-api)
  - [F6 — Signatures are unaffected](#f6--signatures-are-unaffected)
  - [F7 — Upstream pins cosign v2 on main, and there is no newer generator](#f7--upstream-pins-cosign-v2-on-main-and-there-is-no-newer-generator)
  - [F8 — cosign v3 offers no way to read the legacy attachment](#f8--cosign-v3-offers-no-way-to-read-the-legacy-attachment)
  - [F9 — No single cosign version can verify both attachments](#f9--no-single-cosign-version-can-verify-both-attachments)
  - [F10 — Both cosign majors are actively maintained, and cosign-release is settable](#f10--both-cosign-majors-are-actively-maintained-and-cosign-release-is-settable)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [Decision](#decision)
- [Open questions](#open-questions)
- [References](#references)
<!--toc:end-->

## Question

`charts/docz-api/README.md` documents two verification commands for every
published chart and image: a cosign signature check and a SLSA provenance
check. The provenance one fails. **Is our SLSA provenance actually broken, or
is the documented command wrong?**

## Hypothesis

Initial suspicion (recorded before investigating, and **wrong** — see F2/F3):
the `SLSA provenance` jobs report success but the provenance is not being
attached to the digest that `cosign verify-attestation` reads, so the
attestation is effectively missing.

## Context

Found while verifying the freshly published chart `0.3.1` after IMPL-0005 /
PR #15. The signature check passed; the provenance check failed with a
certificate-identity mismatch. Nothing in the recent work touches the publish
pipeline, so the first job was to establish whether this was a regression or a
pre-existing condition.

This matters because the verification commands are **consumer-facing**: they
are in the chart README that operators (and the docz-site) follow, and the
README's "Both are signed with cosign keyless and ship SLSA Level 3 provenance
attestations" claim is a supply-chain promise.

**Triggered by:** post-merge verification of PR #15 (chart 0.3.1); publish
pipeline established in IMPL-0004 Phase 5.

## Approach

1. Run the README's two commands against chart `0.3.1`.
2. Repeat across older published versions to separate regression from
   pre-existing condition.
3. Inspect the registry directly (tag list, manifests, annotations) to find
   where the provenance actually lives.
4. Decode the attestation's signing certificate to read its true SAN and
   Fulcio OID extensions.
5. Re-run the failing command under an older cosign to isolate the variable.

## Environment

| Component | Version / Value |
|-----------|----------------|
| cosign (local, via `mise`) | `v3.1.1` (`mise.toml` pins `cosign = "latest"`) |
| cosign (comparison run) | `v2.6.1` (`mise x aqua:sigstore/cosign@2.6.1`) |
| SLSA generator | `slsa-framework/slsa-github-generator/.github/workflows/generator_container_slsa3.yml@v2.1.0` |
| Signing step | `sigstore/cosign-installer` (pinned SHA) in `.github/workflows/ghcr.yml` |
| Registry | GHCR (`ghcr.io`) |
| Chart under test | `ghcr.io/donaldgifford/charts/docz-api:0.3.1`, digest `sha256:09015dab…7ba8c` |

## Findings

### F1 — The documented command fails on every published version

Not a regression. The signature check passes everywhere; the provenance check
fails everywhere, including versions published well before the current work:

| Chart version | `cosign verify` | `cosign verify-attestation --type slsaprovenance` |
|---------------|-----------------|---------------------------------------------------|
| 0.2.2 | OK | **FAIL** |
| 0.3.0 | OK | **FAIL** |
| 0.3.1 | OK | **FAIL** |

The error is a certificate-identity mismatch, not a missing-attestation error:

```text
Error: no matching attestations: failed to verify certificate identity:
no matching CertificateIdentity found, last error: expected SAN value to match
regex "^https://github.com/slsa-framework/slsa-github-generator/.+", got
"https://github.com/donaldgifford/docz-api/.github/workflows/ghcr.yml@refs/heads/main"
```

The image (`ghcr.io/donaldgifford/docz-api:0.5.0`) behaves identically, so this
is a property of the pipeline, not of the chart path.

### F2 — The provenance exists, is correct, and is signed by the right identity

This **refutes the hypothesis**. Fetching the legacy attestation tag
`sha256-<digest>.att` directly from the registry shows a real SLSA predicate:

```text
layer: application/vnd.dsse.envelope.v1+json
  predicateType = https://slsa.dev/provenance/v0.2
```

Decoding that layer's `dev.sigstore.cosign/certificate` annotation gives a SAN
that **matches the README's regex exactly**:

```text
X509v3 Subject Alternative Name: critical
  URI:https://github.com/slsa-framework/slsa-github-generator/.github/workflows/generator_container_slsa3.yml@refs/tags/v2.1.0
```

with Fulcio extensions confirming the provenance is bound to this repo and
commit: `1.3.6.1.4.1.57264.1.5 = donaldgifford/docz-api`,
`…1.3 = 7c8cc85…` (the PR #15 merge commit),
`…1.18 = …/.github/workflows/release.yml@refs/heads/main`.

So the generator did its job. The artifact is real, correct, and correctly
signed.

### F3 — Two attachment schemes coexist on the same digest

The tag listing for the chart repository shows **two** attachments per digest —
and no `.sig` tag at all:

```text
0.3.1
sha256-09015dab…7ba8c        ← OCI image index, artifactType application/vnd.oci.empty.v1+json
sha256-09015dab…7ba8c.att    ← legacy cosign attestation tag (holds the SLSA provenance)
```

The bare `sha256-<digest>` tag is **cosign v3's referrers-fallback
attachment**; the `.att` tag is the **legacy (cosign v2) attestation tag**.

The split has a simple cause: the two halves of the pipeline run different
cosign majors. `ghcr.yml`'s own signing step uses current cosign (v3) and
writes the new-format attachment; the SLSA generator pins `v2.1.0`, which
bundles cosign v2 and writes the legacy `.att` tag.

cosign v3 reads the new-format attachment, finds only the **signature** there
(signed by `ghcr.yml`, hence the confusing SAN in the F1 error), and never
falls back to the legacy `.att` tag. The provenance is invisible to it.

### F4 — cosign v2 verifies the same artifact with the same command

Decisive. The README's command, unchanged, against the same published chart:

```console
$ mise x aqua:sigstore/cosign@2.6.1 -- cosign verify-attestation \
    --type slsaprovenance \
    --certificate-identity-regexp '^https://github.com/slsa-framework/slsa-github-generator/.+' \
    --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
    ghcr.io/donaldgifford/charts/docz-api:0.3.1
  - Existence of the claims in the transparency log was verified offline
  - The code-signing certificate was verified using trusted certificate authority certificates
Certificate subject: https://github.com/slsa-framework/slsa-github-generator/.github/workflows/generator_container_slsa3.yml@refs/tags/v2.1.0
```

The **only** variable is the cosign major version.

### F5 — GHCR does not serve the OCI referrers API

```console
$ curl .../v2/donaldgifford/charts/docz-api/referrers/sha256:09015dab…
{"errors":[{"code":"MANIFEST_UNKNOWN","message":"manifest unknown"}]}
```

This is why cosign v3 uses the `sha256-<digest>` fallback-tag scheme here
rather than true referrers, and it rules out "just wait for registry support"
as a fix.

### F6 — Signatures are unaffected

`cosign verify` passes on every version with both majors. Only the
**attestation** path is affected. The supply-chain gap is therefore narrow: the
artifacts are signed and the provenance exists — consumers just cannot verify
the provenance with the tool version they will install today.

### F7 — Upstream pins cosign v2 on `main`, and there is no newer generator

The obvious fix — "pin and match the newest majors everywhere" — does not
exist as an option. **Our** cosign is already v3 on both sides (`mise.toml`
and `ghcr.yml`'s signing step). The v2 is inside the reusable workflow, which
hardcodes the binary it runs:

```yaml
# .github/workflows/generator_container_slsa3.yml, at BOTH v2.1.0 and main
- id: cosign-install
  uses: sigstore/cosign-installer@… # v3.7.0 (v3.9.1 on main) — the action
  with:
    cosign-release: v2.2.3          # ← the cosign binary actually used
```

Note the installer *action* is v3.x while the `cosign-release` it installs is
v2.2.3 — easy to misread as "already on v3".

`v2.1.0` (2025-02-24) is the **latest release**; there is nothing newer to bump
to, and `main` has not moved off cosign v2.2.3 either. So no version bump on
our side or theirs changes the attachment scheme. This **eliminates** the
"bump the generator" option that OQ-1 originally listed.

### F8 — cosign v3 offers no way to read the legacy attachment

`cosign verify-attestation --help` on v3.1.1 exposes no
`--attachment-tag-prefix`, no legacy/bundle-format selector, and no relevant
env knob (`cosign env` lists only `COSIGN_EXPERIMENTAL` and
`COSIGN_MAX_ATTACHMENT_SIZE`). There is no v3-only invocation that reaches the
`.att` tag, which rules out fixing this purely in the documented command.

### F9 — No single cosign version can verify both attachments

The split cuts **both** ways, which invalidates the first draft's recommendation
(documenting "use cosign v2" as a complete answer):

| Artifact attachment | cosign v2.6.1 | cosign v3.1.1 |
|---------------------|---------------|---------------|
| Signature (written by our v3 `cosign sign`) | **`no signatures found`** | OK |
| SLSA provenance (written by generator's v2.2.3) | OK | **`no matching attestations`** |

A consumer today needs **two cosign binaries** to check both claims. That is not
a documentable workflow; it is a defect to fix.

### F10 — Both cosign majors are actively maintained, and `cosign-release` is settable

Correcting an assumption in the first draft ("pinning v2 would freeze us on an
old major"): sigstore ships **both** lines concurrently — `v2.6.5` and `v3.1.3`
are both current releases. v2 is not EOL.

And `sigstore/cosign-installer` (we already pin the newest, **v4.1.2**, whose
default is cosign `v3.0.6`) accepts an explicit `cosign-release` input:

```yaml
- uses: sigstore/cosign-installer@…   # v4.1.2
  with:
    cosign-release: 'v2.2.3'          # or any version we choose
```

So the cosign major used by **our** signing steps is a one-line decision, in
`ghcr.yml` (2 call sites) and `ecr.yml` (2 call sites). This is the lever that
makes "align both halves on one scheme" a real option — the first draft treated
our side as immovable, which was wrong.

What remains outside our control is only the **generator's** cosign
(hardcoded `v2.2.3`, per F7).

## Conclusion

**Answer: the documented command is wrong for current cosign; the provenance
itself is fine.**

Nothing is broken in the publish pipeline, and no artifact needs republishing.
The SLSA provenance is generated, correctly scoped to this repo and commit,
signed by the expected `slsa-github-generator` identity, and attached to the
right digest — in the legacy `.att` tag. cosign **v3** (what `mise.toml`'s
`cosign = "latest"` resolves to today, and what a consumer following the README
will install) does not read that tag, so the README's provenance command fails
for everyone.

It is a **tooling-alignment defect**, not a supply-chain defect — but a worse
one than the first pass concluded. Per F9 the split cuts both ways: cosign v3
cannot read the provenance and cosign v2 cannot read the signature, so **no
single cosign version verifies both claims**. "Document the v2 requirement" is
therefore not a fix; it would tell consumers to install two binaries.

The fixable variable is **our own** `cosign-release` (F10), not the generator's
(F7, hardcoded v2.2.3 with no input to override it, on both the latest release
and `main`). So the decision is which scheme to standardise on, and what we are
willing to give up to get there.

## Recommendation

Get the whole pipeline onto current cosign by replacing the reusable SLSA
generator with `actions/attest-build-provenance` (OQ-1a), then add a
post-publish check that runs the documented commands against what was just
shipped (OQ-3) so this cannot silently rot again.

Version bumps alone cannot achieve this: everything we control is already on
the newest v3, and the v2 lives inside upstream's workflow. Re-releasing
without the swap reproduces the same split.

Do **not** retro-fix past versions: their signatures and attestations are each
individually valid; they simply need the right cosign major to check each one.

## Decision

**OQ-1 = (a)**, **OQ-4 = (a)** (chart + image together), decided 2026-08-11.
`slsa-github-generator` is replaced by `actions/attest-build-provenance@v4.2.2`
in both `ghcr.yml` and `ecr.yml`, run as a step inside the existing publish
jobs. The SLSA **Build L2** wording is accepted and the docs now say so rather
than claiming L3.

**OQ-2 = (a)** — `mise.toml` stays `cosign = "latest"`; with the generator gone
everything we publish is written by current cosign, so `latest` is now the
consistent choice rather than the awkward one.

**OQ-3 remains open** — no post-publish self-verification step was added with
this change. It is still the thing that would have caught this originally.

Takes effect from **chart 0.3.2 / image v0.5.1**; earlier artifacts keep the
split and remain verifiable with `cosign v2.x` or `gh attestation verify`.

## Open questions

**OQ-1 — How do we get the whole pipeline onto current cosign?**

Stated goal: one cosign major — the newest v3 line — across `mise.toml`, CI
actions, and everything we publish, cutting fresh artifacts if needed.

Important framing: **we are already there for everything we control.**
`mise.toml` resolves cosign `v3.1.1`; all four `cosign-installer` pins are the
newest `v4.1.2`, whose default is cosign `v3.0.6`. There is no version left to
bump, and **re-releasing changes nothing** — a fresh chart/image would
reproduce the identical split, because the `v2.2.3` is executed inside
`generator_container_slsa3.yml`, which hardcodes it (F7) and exposes no input
to override it. Upstream is not moving either: their only open cosign work is
a Renovate PR *within* the v2 line (v2.6.2).

So uniform-v3 is reachable only by taking the reusable generator out of the
pipeline.

- **(a) Replace the SLSA generator with `actions/attest-build-provenance`.**
  GitHub's native provenance action, `v4.2.2` (2026-08-06), actively
  maintained on current sigstore tooling. It is a drop-in for our two
  provenance jobs: it takes `subject-name` + `subject-digest` (which the chart
  and image jobs already output) and `push-to-registry: true`. Result: every
  signature and attestation we publish is written by current tooling, one
  cosign v3 verifies both, and `gh attestation verify` works too. Requires
  cutting a new chart/image to take effect — previously published artifacts
  keep their split and stay individually valid. **← recommendation**
  - **Caveat to accept knowingly:** provenance would be signed by *our*
    repository's Actions identity rather than a separate trusted-builder
    reusable workflow. GitHub's build attestations are SLSA v1 **Build L2**;
    the reusable generator's pitch was L3 non-falsifiability. The chart
    README's "ship SLSA Level 3 provenance attestations" line must be
    corrected to match — the honest trade is *current, verifiable tooling* over
    *a level claim nobody can currently verify*.
- (b) Align down instead: pin our `cosign-release` to `v2.2.3` to match the
  generator, keeping SLSA L3. One cosign v2 then verifies both, and the
  README's existing commands work unchanged. Rejected against the stated goal
  — it moves us *off* current tooling, and consumers on the v3 default (the
  installer's own default) could not verify us.
- (c) Keep both schemes and document two binaries (v3 for the signature, v2 for
  the provenance). Zero pipeline change, bad consumer experience.
- (d) Keep the signature, drop the provenance job and its README claim until
  upstream ships a v3-writing generator — which, per the issue tracker, is not
  in progress.

**OQ-2 — Do we pin cosign in `mise.toml` instead of `latest`?**

- **(a) Leave `cosign = "latest"`.** (Weakened by F9/F10 — see (b).) The local pin drives signing and ad-hoc
  checks, not consumer behaviour, and Renovate keeps it current. Pinning to v2
  to make one doc command work would freeze us on an old major. **←
  recommendation**
- (b) Pin to whatever major OQ-1 settles on, so local `cosign` matches what CI
  writes and what the README tells consumers to run. If OQ-1(a) wins this is
  the more consistent choice — `latest` would leave the local binary unable to
  verify our own artifacts.
- (c) Pin to a specific v3.x for reproducibility and document the v2 fallback
  separately.

**OQ-3 — Should CI verify its own published artifacts?**

- **(a) Add a post-publish verification step to `ghcr.yml`** that runs both
  documented commands against the just-published digest and fails the job on
  mismatch. This defect survived every release because nothing ever executed
  the README's commands. Cheap, and it turns the README into a tested claim.
  **← recommendation**
- (b) Add it as a scheduled workflow against the latest published version
  instead, keeping the release path fast.
- (c) Skip — treat verification as a consumer concern.

**OQ-4 — Scope of the fix.**

- **(a) Chart README + image docs together**, since `ghcr.io/donaldgifford/
  docz-api` has the identical split and the same README section covers both.
  **← recommendation**
- (b) Chart only for now; handle the image when someone asks.

## References

- `charts/docz-api/README.md.gotmpl` — the "Verifying the chart" section
  containing both commands (lines ~178–200).
- `.github/workflows/ghcr.yml` — `chart` / `image` publish jobs and the two
  `SLSA provenance` jobs (`generator_container_slsa3.yml@v2.1.0`).
- IMPL-0004 Phase 5 — where the publish + signing pipeline was consolidated.
- PR #15 — the chart 0.3.1 publish whose verification surfaced this.
- [OCI referrers API](https://github.com/opencontainers/distribution-spec/blob/main/spec.md#listing-referrers)
- [slsa-github-generator container generator](https://github.com/slsa-framework/slsa-github-generator/blob/main/internal/builders/container/README.md)
