---
id: DOC-0000
title: "Frontmatter Title That Title() Never Reads"
status: Draft
author: Test Author
created: 2026-08-11
---
<!-- markdownlint-disable-file MD025 MD041 MD024 -->

Prose before any heading, so the title is not on line 1 and a walker that
assumes it is will report nothing.

```md
# Fenced Decoy Title

Everything in here is a code sample, including the heading above.
```

# **Real** `Title` With [Markup](https://example.com)

## A Section After The Title

- [ ] a task, so this fixture exercises all three walkers
- [x] a finished one

---

# A Second H1 That Loses To The First

Setext Heading That Also Loses
==============================

## Another Section

    # Indented four spaces, and still a heading to both walkers —
    # docparse does not model indented code blocks. This one is an H1,
    # so Headings skips it and Title has already found the real title.
