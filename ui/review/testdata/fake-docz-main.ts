// The fake docz's process: answers each subcommand (or subcommand and
// first argument) from the config the test wrote, stepping through a
// list of responses per key, and
// logs every call (argv and cwd) for the test to read back.
import { appendFileSync, readFileSync, writeFileSync } from "node:fs";

interface Response {
  code?: number;
  stdout?: string;
  stderr?: string;
}

const configPath = process.env.FAKE_DOCZ_CONFIG ?? "";
const config = JSON.parse(readFileSync(configPath, "utf8")) as Record<
  string,
  Response[] | undefined
>;
const args = process.argv.slice(2);
const cmd = args[0] ?? "";
appendFileSync(
  `${configPath}.log`,
  `${JSON.stringify({ args, cwd: process.cwd() })}\n`,
);

const counter = `${configPath}.${encodeURIComponent(`${cmd} ${args[1] ?? ""}`)}.n`;
let n = 0;
try {
  n = Number(readFileSync(counter, "utf8"));
} catch {
  // first call
}
writeFileSync(counter, String(n + 1));

// "inspect DESIGN-0001" answers that argument; "inspect" any other.
const list = config[`${cmd} ${args[1] ?? ""}`] ??
  config[cmd] ?? [{ code: 2, stderr: `fake docz: no ${cmd}\n` }];
const res = list[Math.min(n, list.length - 1)] ?? {};
process.stdout.write(res.stdout ?? "");
process.stderr.write(res.stderr ?? "");
process.exit(res.code ?? 0);
