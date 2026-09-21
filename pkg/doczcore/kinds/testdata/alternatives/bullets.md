## Alternatives Considered

- **A. Promote on demand, again.** Ship `pkg/impl` and the two `docwrite`
  byte cores for tempy and stop (DESIGN-0013 Phase A). *Pros:* smallest
  change. *Cons:* leaves the orchestration gap that produced the re-implementation
  in the first place; the next consumer gets the next drip; the CLI never
  becomes a consumer.
- **B. Typed models inside `doczcore`** (`pkg/doczcore/impl`). *Pros:* one
  import root. *Cons:* every consumer of the core compiles every type model;
  the core stops being type-agnostic; the "pass-through" story for custom
  types gets murky. Rejected in the DESIGN-0013 review.
- **C. A runtime type system** (go-cty style, schema-driven types so custom
  types get models too). *Pros:* uniform. *Cons:* Go consumers want concrete
  structs; docz's grammars are fixed by its own templates; heavy machinery
  for six types.
- **D. Swap `cmd/` incrementally, command by command, releasing as we go.**
  *Pros:* smaller PRs. *Cons:* the API is designed around what each command
  needs in isolation rather than as one unit, which is how today's shape
  happened; partial swaps leave `cmd/` on two code paths for months.
- **E. A separate library module.** Rejected in ADR-0001 and again here
  (Decision 3).
- **F. This ADR.** API first, layers and rules, one module, standalone type
  packages with `Doc`, primitives versus policy, whole API then swap.
