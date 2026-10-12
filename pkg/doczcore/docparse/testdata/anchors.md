<!-- markdownlint-disable-file MD003 MD024 MD033 MD042 MD049 -->

# Anchor ground truth

Every heading below is followed by a link to the anchor GitHub,
markdownlint MD051, and github-slugger give it. The links are written by
hand and checked by markdownlint (`just lint-generated`), and
`TestAnchorsMatchGitHub` checks that docz's ToC agrees with each one.

One case is not here: a literal `## Dup-1` beside generated `dup-1`
suffixes. github-slugger, which docz-site's `rehype-slug` uses, skips a
taken suffix, and markdownlint's MD051 does not, so no link can satisfy
both. docz follows github-slugger, pinned by `TestHeadingsDuplicateCollision`.

## The id_prefix rule

[this heading](#the-id_prefix-rule)

## Use a_b_c names

[this heading](#use-a_b_c-names)

## Metrics: our own exporter (technitium_exporter)

[this heading](#metrics-our-own-exporter-technitium_exporter)

## Über uns

[this heading](#über-uns)

## Café résumé

[this heading](#café-résumé)

## Version 2.0.1 released

[this heading](#version-201-released)

## What?! Really... yes

[this heading](#what-really-yes)

## Dots in foo.bar and a/b

[this heading](#dots-in-foobar-and-ab)

## Phase 1: <!-- Foundation / Setup / Core -->

[this heading](#phase-1-)

## Foo <!-- c --> bar

[this heading](#foo--bar)

## <!-- only a comment -->

[this heading](#)

## The `code_span` heading

[this heading](#the-code_span-heading)

## **Bold** and *italic* and _under_

[this heading](#bold-and-italic-and-under)

## A [docs link](https://example.com) here

[this heading](#a-docs-link-here)

## An <code>inline</code> tag

[this heading](#an-inline-tag)

## Closing sequence ##

[this heading](#closing-sequence)

## -leading and trailing-

[this heading](#-leading-and-trailing-)

## Emoji 🎉 party

[this heading](#emoji--party)

## Dup

[this heading](#dup)

## Dup

[this heading](#dup-1)

## Dup

[this heading](#dup-2)

<!--
## Hidden by a comment block

No renderer emits this heading, so docz gives it no ToC entry.
-->

```markdown
## Inside a fence
```
