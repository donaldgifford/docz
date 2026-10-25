// A session with documents armed and a fake docz that answers for them,
// shared by the round and server tests.
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import type { DoczInspect } from "@/cli/__generated__/inspect";
import type { Finding } from "@/cli/__generated__/validate";
import { FINDINGS, INSPECT } from "@/review/testing";

import { Docz } from "../docz";
import { type Entry, Session } from "../state";
import {
  type FakeConfig,
  fakeDocz,
  validateOutput,
  version,
} from "./fake-docz";

export interface FixtureDoc {
  inspect: DoczInspect;
  findings: Finding[];
}

/** A copy of the fixture document under another id and path. */
export function docAs(id: string, path: string, content?: string): FixtureDoc {
  const inspect: DoczInspect = {
    ...INSPECT,
    id,
    path,
    frontmatter:
      INSPECT.frontmatter === null ? null : { ...INSPECT.frontmatter, id },
  };
  if (content !== undefined) {
    inspect.content = content;
    inspect.content_sha256 = new Bun.CryptoHasher("sha256")
      .update(content)
      .digest("hex");
  }
  return { inspect, findings: FINDINGS };
}

/** Responses for docs, each answering by its path and by its id. */
export function doczConfig(docs: readonly FixtureDoc[]): FakeConfig {
  const config: FakeConfig = { version: version() };
  for (const d of docs) {
    const out = { stdout: JSON.stringify(d.inspect) };
    config[`inspect ${d.inspect.path}`] = [out];
    config[`inspect ${d.inspect.id}`] = [out];
    config[`validate ${d.inspect.path}`] = [
      validateOutput(d.inspect, d.findings),
    ];
  }
  return config;
}

export function reviewFixture(
  docs: readonly FixtureDoc[],
  sessionId = "sess-1",
) {
  const base = mkdtempSync(join(tmpdir(), "review-state-"));
  const root = mkdtempSync(join(tmpdir(), "review-repo-"));
  const fake = fakeDocz(doczConfig(docs));
  const docz = new Docz(fake.bin);
  const session = new Session({ base, root, session: sessionId });
  const entries: Entry[] = docs.map((d) =>
    session.arm({
      id: d.inspect.id,
      type: d.inspect.type,
      path: d.inspect.path,
      root,
      config: "",
      docs_dir: "",
    }),
  );
  return { base, root, fake, docz, session, entries };
}
