# The hook drill

IMPL-0027 Phase 1 settles by observation what DESIGN-0024 could not
settle from Claude Code's documentation. The planned drill is a
throwaway plugin of shell scripts, one case per question. This record
holds a **partial** run: a trial of docz-review itself on 2026-10-27,
whose session transcript and a `claude --debug` log answer some of the
cases. The rest stay open until the drill plugin runs.

Evidence was read from the session transcript
(`~/.claude/projects/<repo>/<session>.jsonl`), the debug log
(`claude --debug`, `~/.claude/debug/<session>.txt`), and docz-review's
state directory. Nothing below is inferred from documentation.

## Results

| Case | Question | Observation | Claude Code | Date |
| --- | --- | --- | --- | --- |
| 1 | Does a `Stop` hook's stdin reach EOF? | Yes. `docz-review --pending --hook` reads stdin to EOF before it serves, and every run opened the page; the `PostToolUse` arm hook, which also reads to EOF, armed after `docz create` and `docz-review arm` | 2.1.280 | 2026-10-27 |
| 2 | The default 600-second timeout | Not observed | — | — |
| 3 | A configured timeout, and any ceiling | Partly. With `timeout: 86400` the host recorded `timeoutMs: 86400000` on the hook, with no warning or clamp. No run lasted longer than about three minutes, so the ceiling itself is not measured | 2.1.280 | 2026-10-27 |
| 4 | Block and continue | Yes. `{"decision":"block","reason":…}` is recorded as `hook_blocking_error` with the reason, and the agent resumed with it, three times in one session. The person sees the reason labelled "Stop hook error:", which the README now explains; whether a `systemMessage` beside a block is shown is not recorded | 2.1.280 | 2026-10-27 |
| 5 | Repeated blocks with no tool call between | Not observed; each block in the trial was followed by tool calls | — | — |
| 6 | Interrupt | Escape: recorded as `hook_cancelled` (`timedOut: false`, after 50.6 s). docz-review's signal handler ran: the open document was `dismissed` within the same minute rather than after the 120-second grace, and the lease was released, since an immediate re-arm opened round 3. Ctrl-C not observed | 2.1.280 | 2026-10-27 |
| 7 | `SIGKILL` from outside | Not observed | — | — |
| 8 | `tool_response` shapes | Not captured; a hook's stdin is not in the transcript, so the fixtures in `ui/review/testdata/hook/` stay hand-written | — | — |
| 9 | The `if` field | Works on the handler (beside `type` and `command`); on the matcher group the host warns `unknown keys "if"` and ignores it. The debug log's `Skipping hook due to if condition` lines show `Bash(docz *)` skipped `echo docz` and matched `ls && docz list` and `FOO=1 docz list`, so it is evaluated per subcommand with a leading assignment ignored | 2.1.280 | 2026-10-27 |
| 10 | An echoed `Created …` line | Not observed | — | — |

## What it changed

- `hooks/hooks.json` moved `if` from the matcher group to each handler
  (commit `c36a886`); before that the arm hook ran on every `Bash` call
  and stayed correct only because `arm.sh` checks the command itself.
- A comment selected across a heading took in the heading's `#` copy
  button, so its quote was never in the source and its anchor resolved
  `lost` (commit `5b202ed`).

- Without the binary, the one install line both hooks wrote to stderr
  on exit 0 reached only the debug log, so a person never saw it.
  `arm.sh` now prints it as a `systemMessage` after a `docz create`;
  `stop.sh` stays on stderr, since nothing can have been armed.

## Also seen

- A failed command fires `PostToolUseFailure`, not `PostToolUse`, so
  `docz-review arm` run without the binary (exit 127) never reaches
  `arm.sh`; the agent reads the shell's "command not found" instead.

- A plugin installed from a local-path marketplace is read from the
  working tree (`Read hooks.json for plugin docz-review …
  /plugins/docz-review/hooks/hooks.json`), so an edit there needs a
  reload, not a reinstall.
- The minimum Claude Code version is 2.1.280 until an older one is
  tried: it is the only version observed, and it honours both the `if`
  filter and the 86400-second timeout.
