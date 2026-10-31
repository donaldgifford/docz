# The hook drill

IMPL-0027 Phase 1 settles by observation what DESIGN-0024 could not
settle from Claude Code's documentation. It ran twice:

- **A trial of docz-review itself** on 2026-10-27, whose session
  transcript and `claude --debug` log answered case 9 and part of cases
  1, 3, 4, and 6.
- **The drill plugin** on 2026-10-29: a throwaway local marketplace,
  never committed, with a `PostToolUse` hook on `Bash` that appended its
  stdin to a log and a `Stop` hook script that logged its stdin, every
  signal it caught (`TERM`, `HUP`, `INT`), and each step of a configured
  sleep, then printed a configured reply. Each case was a configuration
  file, and the logs were read after each one.

Both ran on Claude Code 2.1.280 on macOS. Nothing below is inferred
from documentation.

## Results

| Case | Question | Observation | Claude Code | Date |
| --- | --- | --- | --- | --- |
| 1 | Does a `Stop` hook's stdin reach EOF? | Yes, at once: every drill run logged EOF in the same second it started, with requests of 472 to 1,335 bytes. The trial's `docz-review --pending --hook` also read to EOF before it served | 2.1.280 | 2026-10-29 |
| 2 | The default 600-second timeout | With no `timeout` on the entry, a hook sleeping 700 s got `TERM` at exactly 600 s; its child `sleep` got it too, so the whole process group is signalled. A reply printed after the signal is discarded | 2.1.280 | 2026-10-29 |
| 3 | A configured timeout, and any ceiling | `timeout: 1200` and `timeout: 86400` each let a 700 s sleep finish without a signal, and the block that followed was honoured. The host records `timeoutMs: 86400000` with no warning or clamp. No run lasted longer than 700 s, so a ceiling between 700 s and 24 hours is not ruled out | 2.1.280 | 2026-10-29 |
| 4 | Block and continue | `{"decision":"block","reason":…,"systemMessage":…}`: the person sees both, the system message as `Stop says: <message>` and the reason as `Stop hook error: <reason>`; the agent receives only the reason and resumes with it. The hook then runs again with `stop_hook_active: true` | 2.1.280 | 2026-10-29 |
| 5 | Repeated blocks | `stop_hook_active` is in every request: `false` on the first, `true` on each one after a block. With no tool call between blocks the host accepted eight and overrode the ninth, ending the turn with "A hook blocked the turn from ending 9 consecutive times", and named `CLAUDE_CODE_STOP_HOOK_BLOCK_CAP` to raise the limit. With one `Bash` call between blocks, fifteen in a row were all honoured, so a tool call resets the count | 2.1.280 | 2026-10-29 |
| 6 | Interrupt | Escape (trial) and Ctrl-C (drill) both cancel the hook with a signal it can catch: for Ctrl-C, `TERM` to its process group; for Escape, recorded as `hook_cancelled`, a signal docz-review caught. A reply printed after the signal is discarded and the session shows "Interrupted". docz-review's handler dismissed the open document at once and released the lease | 2.1.280 | 2026-10-29 |
| 7 | `SIGKILL` from outside | The host waits for the hook's stdout to close, not for the process: with the script killed and its `sleep` child still holding the pipe, the session waited until the child ended. Then it showed `Stop hook error: Failed with non-blocking status code: No stderr output` and let the turn end. `stop.sh` `exec`s the binary, which holds the pipe alone, so a killed docz-review ends the turn at once | 2.1.280 | 2026-10-29 |
| 8 | `tool_response` shapes | `{stdout, stderr, interrupted, isImage, noOutputExpected}`, no exit code; a command that wrote files adds `bashEditDiff` with each file's full diff; `stdout` is trimmed and capped at 30,000 characters, with the rest in `persistedOutputPath`. A failed command fires `PostToolUseFailure`, not `PostToolUse`. A compound command is one request. Captures are `ui/review/testdata/hook/captured-*.json` | 2.1.280 | 2026-10-29 |
| 9 | The `if` field | Works on the handler (beside `type` and `command`); on the matcher group the host warns `unknown keys "if"` and ignores it. `Bash(docz *)` skipped `echo docz` and matched `ls && docz list` and `FOO=1 docz list`, so it is evaluated per subcommand with a leading assignment ignored | 2.1.280 | 2026-10-27 |
| 10 | An echoed `Created …` line | The response carries the line verbatim. Fed to `docz-review arm --from-hook`, the captured request armed nothing (no `docz` word in the command), and the plugin's `if` filter would not have run the hook at all | 2.1.280 | 2026-10-29 |

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
- The arming fixtures gained the real captures, and a test pins that a
  `Created` line or a JSON envelope inside `bashEditDiff` is never a
  candidate: every diff line starts with `+`, `-`, or a space.

## What it confirmed

- **The deadline.** The server's default deadline of the hook timeout
  minus sixty seconds stands. A timeout above 600 s is honoured, and a
  hook that overruns gets `TERM`, the signal docz-review already treats
  as dismiss, so even a missed deadline ends in a recorded outcome.
- **The round limit.** DESIGN-0024 question 4's `--max-rounds` (default
  10) is what bounds a review: the agent edits between rounds, and a
  tool call resets the host's count. The host's cap only ends a turn
  in which the agent answers nine rounds in a row without a tool call;
  the review stays armed and opens on the next turn.
- **`stop_hook_active`.** docz-review must not return early when it is
  `true`, as the host's cap message suggests: every round after the
  first arrives with it set.

## Also seen

- A plugin installed from a local-path marketplace is read from the
  working tree, so an edit there needs `/reload-plugins`, not a
  reinstall.
- Other plugins' `Stop` hooks run beside docz-review's ("Ran 3 stop
  hooks").
- The minimum Claude Code version is 2.1.280 until an older one is
  tried: it is the only version observed.
