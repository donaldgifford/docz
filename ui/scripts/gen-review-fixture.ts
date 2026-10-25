/*
 * Captures `docz inspect` and `docz validate` for every document in the
 * review fixture repository (e2e/fixtures/review-repo) into
 * e2e/fixtures/review-session.json, which `just ui dev-review` and the
 * review e2e spec serve through the fake server. Committed, so neither
 * needs a docz binary; cmd/review_fixture_test.go fails when the
 * capture no longer matches what docz prints, and this script is the
 * fix: `bun run gen-review-fixture` after `just build`.
 *
 * The docz binary is DOCZ_REVIEW_DOCZ, else ../build/bin/docz.
 */
import { execFile } from "node:child_process";
import { readdir, writeFile } from "node:fs/promises";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

const run = promisify(execFile);
const here = fileURLToPath(new URL(".", import.meta.url));
const root = join(here, "..", "e2e", "fixtures", "review-repo");
const out = join(here, "..", "e2e", "fixtures", "review-session.json");
const docz =
  process.env.DOCZ_REVIEW_DOCZ ??
  join(here, "..", "..", "build", "bin", "docz");

/** The version stamp a capture carries, so a rebuild is not drift. */
export const FIXTURE_VERSION = "fixture";

async function documents(dir: string): Promise<string[]> {
  const entries = await readdir(dir, { withFileTypes: true });
  const nested = await Promise.all(
    entries.map(async (e) => {
      const path = join(dir, e.name);
      if (e.isDirectory()) {
        return documents(path);
      }
      return /^\d{4}-.*\.md$/.test(e.name) ? [path] : [];
    }),
  );
  return nested.flat().sort();
}

// validate exits 1 when it reports findings; its stdout is still the report.
async function json(args: string[]): Promise<unknown> {
  try {
    const { stdout } = await run(docz, args, { maxBuffer: 16 << 20 });
    return JSON.parse(stdout);
  } catch (err) {
    const stdout = (err as { stdout?: string }).stdout ?? "";
    if (stdout === "") {
      throw err;
    }
    return JSON.parse(stdout);
  }
}

const session = [];
for (const abs of await documents(join(root, "docs"))) {
  const path = relative(root, abs);
  const inspect = (await json([
    "inspect",
    path,
    "--repo-root",
    root,
    "--format",
    "json",
  ])) as Record<string, unknown>;
  inspect.docz_version = FIXTURE_VERSION;
  const validate = (await json([
    "validate",
    path,
    "--repo-root",
    root,
    "--format",
    "json",
  ])) as { docs?: { findings?: unknown[] }[] };
  session.push({
    inspect,
    findings: validate.docs?.[0]?.findings ?? [],
  });
}

await writeFile(out, `${JSON.stringify(session, null, 2)}\n`);
console.log(
  `wrote ${relative(process.cwd(), out)}: ${String(session.length)} documents`,
);
