# Table shapes

## The File Changes shape

| File | Action | Description |
| ---- | ------ | ----------- |
| `regions.go` | Add | The region walker |
| `doc.go` | Modify | Package comment |

## Alignment colons and no outer pipes

| Left | Center | Right |
|:-----|:------:|------:|
| a | b | c |

Component | Value
--------- | -----
Go | 1.26.4
OS | darwin/arm64

## Ragged rows are reported as written

| A | B | C |
| - | - | - |
| only one |
| one | two | three | four |

## Inline markdown is kept

| Risk | Mitigation |
| ---- | ---------- |
| **Bold** risk | See [ADR-0002](../adr/0002.md) |
| A `code` risk | Escaped pipe: a \| b |

## Empty cells

| A | B |
| - | - |
|  |  |
| x |  |

## Not tables

A paragraph with a | pipe in it but no delimiter row.

| Header only |

| A | B |
| not | a delimiter |
| x | y |

## Adjacent tables with no blank line are one table

A row is a row: without a blank line the second header and its delimiter
are body rows of the first table, which is what GFM renders too.

| One |
| --- |
| 1 |
| Two |
| --- |
| 2 |

## Separated by a blank line they are two

| One |
| --- |
| 1 |

| Two |
| --- |
| 2 |

## Inside a fence

```markdown
| Not | A | Table |
| --- | - | ----- |
| a | b | c |
```

| After | The fence |
| ----- | --------- |
| real | table |
