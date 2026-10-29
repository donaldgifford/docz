# Hook request fixtures

`PostToolUse` requests for the `Bash` tool, as `docz-review arm
--from-hook` reads them on stdin, for `review/arm.test.ts` and the
plugin's hook tests.

The `captured-*` files are real requests from the IMPL-0027 Phase 1
drill on Claude Code 2.1.280 (2026-10-29), recorded by a `PostToolUse`
hook and changed only to replace the home directory, the session id,
and the author's name. They show the shape the host sends:

- `tool_response` is `{stdout, stderr, interrupted, isImage,
  noOutputExpected}`, with no exit code. A command that wrote files adds
  `bashEditDiff`, the full diff of each, every line prefixed `+`, `-`,
  or a space, so a document's own text never starts a line of the
  response.
- `stdout` is trimmed of its trailing newline and capped at 30,000
  characters; the rest is in a file named by `persistedOutputPath`, with
  `persistedOutputSize` the full length (`captured-long-output`).
- A command that fails fires `PostToolUseFailure`, which the plugin does
  not hook, so a failed `docz create` never reaches arming.

The other files are hand-written for cases the drill could not produce
or that the tests need under fixed names. `long-output` is cut to the
30,000-character cap, and `create-failure` is a request the host does
not send, kept as a defence in case a later version does.
