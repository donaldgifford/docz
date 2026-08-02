# Edge Cases

Prose in the preamble.

## [unreleased]

Content inside a version before any group heading.

- bullet with no group

### Features

- item one
  continued on the next line

- item two

  after a blank line, still item two
  - nested sub-bullet, marker kept
- item three

Column-0 prose between bullets is dropped.

### Features

- duplicate group title, separate entry

## [v1.0.0] - 2026-01-01

## [1.0.0] - 2026-01-01

### Bug Fixes

- duplicate version identity after the v-trim
- fenced decoy below stays inside this item

  ```markdown
  ## [9.9.9] - 2099-01-01
  ### Not A Group
  - not an item
  ```

- *star bullet* item
-
