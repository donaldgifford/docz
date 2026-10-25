# Hook request fixtures

`PostToolUse` requests for the `Bash` tool, as `docz-review arm
--from-hook` reads them on stdin, for `review/arm.test.ts`.

They are written by hand in the shape Claude Code documents
(`tool_response` as `{stdout, stderr, interrupted, isImage}`), because
the IMPL-0027 Phase 1 drill that captures real ones is deferred until a
person runs it. When it runs, replace these with its captures under the
same names; the parser already reads every string in `tool_response`,
so a different shape changes the fixtures, not the code.
