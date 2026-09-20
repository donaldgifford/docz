# List shapes

Every bullet style, numbering style, nesting depth, and near miss the
fleet's documents actually contain.

## Bullets

- A dash item.
* A star item.
+ A plus item.

- **Bold lead-in:** the rest of the sentence.
- An item with `code` and a [link](https://example.invalid).
  - Nested two spaces.
    - Nested four.
	- Nested with a tab, which counts as one.

## Numbered

1. First.
2. Second.
3) Closing paren style.
10. Double digits.

1. An ordered item with a nested unordered one.
   - Inside.

## Checkbox items are list items too

- [ ] An unchecked task.
- [x] A checked task.
- [-] A bracket docz does not treat as a checkbox.

## Empty and near misses

-
- 

Not a list:

-text
1.text
---
***
___
  ---

A line that merely contains - a dash.

## Inside a fence

```markdown
- this is an example, not an item
1. nor this
```

- After the fence, a real item.
