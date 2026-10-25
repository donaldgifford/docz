/*
 * A Vite plugin that serves docz-review's API from the in-process fake
 * (src/review/fake-server.ts) over the committed fixture capture
 * (e2e/fixtures/review-session.json), for `just ui dev-review` and the
 * review e2e spec. It runs only in `vite --mode review`'s dev server;
 * the real server is ui/review/server.ts.
 *
 * Beside the API it answers three test hooks, unauthenticated because
 * they exist only here: GET /__fake/records (the review records so far),
 * GET /__fake/feedback (the round's message for them), and
 * POST /__fake/reset (a fresh fake).
 */
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { FakeReviewServer } from "../src/review/fake-server";
import { feedbackMessage } from "../src/review/feedback";

import type { DoczInspect } from "../src/cli/__generated__/inspect";
import type { Finding } from "../src/cli/__generated__/validate";
import type { ReviewDocument } from "../src/review/api";
import type { IncomingMessage, ServerResponse } from "node:http";
import type { Plugin } from "vite";

export const DEV_TOKEN =
  process.env.DOCZ_REVIEW_DEV_TOKEN ?? "dev-review-token";

interface Captured {
  inspect: DoczInspect;
  findings: Finding[];
}

const capture = fileURLToPath(
  new URL("../e2e/fixtures/review-session.json", import.meta.url),
);

function load(): Captured[] {
  return JSON.parse(readFileSync(capture, "utf8")) as Captured[];
}

function documents(entries: Captured[]): ReviewDocument[] {
  return entries.map(({ inspect, findings }) => ({
    key: inspect.id,
    round: `round-${inspect.id}-1`,
    round_number: 1,
    state: "open",
    inspect,
    findings,
    previous: null,
    draft: null,
  }));
}

async function body(req: IncomingMessage): Promise<Buffer> {
  const chunks: Buffer[] = [];
  for await (const chunk of req) {
    chunks.push(chunk as Buffer);
  }
  return Buffer.concat(chunks);
}

async function send(res: ServerResponse, out: Response): Promise<void> {
  res.statusCode = out.status;
  out.headers.forEach((value, key) => {
    res.setHeader(key, value);
  });
  if (out.body === null) {
    res.end();
    return;
  }
  const reader = out.body.getReader();
  res.on("close", () => {
    reader.cancel().catch(() => undefined);
  });
  for (;;) {
    const { done, value } = await reader.read();
    if (done) {
      res.end();
      return;
    }
    res.write(value);
  }
}

export function reviewFake(): Plugin {
  const entries = load();
  let fake = new FakeReviewServer({
    token: DEV_TOKEN,
    documents: documents(entries),
  });

  return {
    name: "docz-review-fake",
    apply: "serve",
    configureServer(server) {
      server.httpServer?.once("listening", () => {
        const address = server.httpServer?.address();
        const port =
          typeof address === "object" && address !== null ? address.port : 5173;
        server.config.logger.info(
          `  docz-review (fake): http://127.0.0.1:${String(port)}/review.html?t=${DEV_TOKEN}`,
        );
      });
      server.middlewares.use((req, res, next) => {
        const url = req.url ?? "";
        if (url === "/__fake/reset" && req.method === "POST") {
          fake = new FakeReviewServer({
            token: DEV_TOKEN,
            documents: documents(entries),
          });
          res.end("{}");
          return;
        }
        if (url === "/__fake/records") {
          res.setHeader("Content-Type", "application/json");
          res.end(JSON.stringify(fake.records));
          return;
        }
        if (url === "/__fake/feedback") {
          const message = feedbackMessage(
            fake.records.map((record) => {
              const entry = entries.find(
                (e) => e.inspect.id === record.document.id,
              );
              return {
                record,
                title:
                  entry?.inspect.frontmatter?.title ?? record.document.path,
                currentStatus: entry?.inspect.frontmatter?.status ?? "",
                questions: entry?.inspect.open_questions ?? [],
                findings: entry?.findings ?? [],
              };
            }),
          );
          res.setHeader("Content-Type", "text/plain; charset=utf-8");
          res.end(message);
          return;
        }
        if (!url.startsWith("/api/")) {
          next();
          return;
        }
        void (async () => {
          const headers = new Headers();
          for (const [k, v] of Object.entries(req.headers)) {
            if (typeof v === "string") {
              headers.set(k, v);
            }
          }
          const hasBody = req.method !== "GET" && req.method !== "HEAD";
          const request = new Request(`http://127.0.0.1${url}`, {
            method: req.method ?? "GET",
            headers,
            body: hasBody ? new Uint8Array(await body(req)) : undefined,
          });
          await send(res, await fake.handle(request));
        })().catch((err: unknown) => {
          next(err);
        });
      });
    },
  };
}
