import { describe, expect, test } from "bun:test";

import { Ajv2020 } from "ajv/dist/2020";

import reviewSchema from "../../api/cli/review.schema.json";

import { parseReview } from "@/cli/validate";
import { INSPECT } from "@/review/testing";

import { Review } from "./round";
import { doczConfig, docAs, reviewFixture } from "./testdata/review-fixture";

const A = docAs(INSPECT.id, INSPECT.path);

const ajv = new Ajv2020({ allErrors: true, validateFormats: false });
ajv.addKeyword("$version");
const closedReview = ajv.compile(reviewSchema);

async function opened(docs = [A], maxRounds = 5) {
  const fx = reviewFixture(docs);
  const res = await Review.open(fx.entries, {
    session: fx.session,
    docz: fx.docz,
    reviewId: "sess-1",
    deadline: new Date("2026-10-25T16:00:00Z"),
    maxRounds,
    now: () => new Date("2026-10-25T15:04:05.123Z"),
  });
  const doc = res.review.view().documents[0];
  if (doc === undefined) {
    throw new Error("nothing opened");
  }
  const key = (submission_id: string) => ({
    round: doc.round,
    content_sha256: doc.inspect.content_sha256,
    submission_id,
  });
  return { ...fx, ...res, doc, key };
}

const changes = {
  outcome: "changes",
  questions: [{ number: 1, choice: "b" }],
  annotations: [
    { order: 1, kind: "document", text: "Shorten the background." },
  ],
  status: "In Review",
  findings: [{ code: "content.not-bullets", line: 11, action: "fix" }],
};

describe("Review.open", () => {
  test("opens round 1 from a snapshot and keeps it for the next diff", async () => {
    const { doc, session, entries } = await opened();
    expect(doc).toMatchObject({
      key: A.inspect.id,
      round_number: 1,
      state: "open",
    });
    expect(doc.previous).toBeNull();
    expect(session.read().entries[0]?.state).toBe("open");
    expect(session.readSnapshot(entries[0]?.key ?? "", 1)).toBe(
      A.inspect.content,
    );
  });

  test("dismisses a document at the round limit instead of opening it", async () => {
    const fx = reviewFixture([A]);
    const key = fx.entries[0]?.key ?? "";
    fx.session.apply(key, "open");
    fx.session.apply(key, "changes");
    const res = await Review.open(fx.session.read().entries, {
      session: fx.session,
      docz: fx.docz,
      reviewId: "r",
      deadline: new Date(),
      maxRounds: 1,
    });
    expect(res.review.size).toBe(0);
    expect(res.limited.map((e) => e.state)).toEqual(["dismissed"]);
  });

  test("re-arming lifts the round limit and keeps counting rounds", async () => {
    const fx = reviewFixture([A]);
    const key = fx.entries[0]?.key ?? "";
    fx.session.apply(key, "open");
    fx.session.apply(key, "changes");
    const entry = fx.entries[0];
    if (entry === undefined) {
      throw new Error("no entry");
    }
    fx.session.arm(entry);
    const res = await Review.open(fx.session.read().entries, {
      session: fx.session,
      docz: fx.docz,
      reviewId: "r",
      deadline: new Date(),
      maxRounds: 1,
    });
    expect(res.limited).toEqual([]);
    expect(res.review.view().documents[0]?.round_number).toBe(2);
  });

  test("a document docz cannot read is reported and left as it was", async () => {
    const fx = reviewFixture([A]);
    fx.fake.set({});
    const res = await Review.open(fx.entries, {
      session: fx.session,
      docz: fx.docz,
      reviewId: "r",
      deadline: new Date(),
      maxRounds: 5,
    });
    expect(res.failed).toHaveLength(1);
    expect(fx.session.read().entries[0]?.state).toBe("pending");
  });
});

describe("Review.record", () => {
  test("records changes, fsyncs the record, then moves the state", async () => {
    const { review, key, session, entries } = await opened();
    const reply = await review.record(A.inspect.id, "submit", {
      ...key("s_1"),
      ...changes,
    });
    expect(reply).toEqual({
      status: 200,
      body: { outcome: "changes", state: "changes" },
    });
    const rec = session.readRecord(entries[0]?.key ?? "", 1);
    expect(parseReview(JSON.stringify(rec))).toMatchObject({
      review_id: "sess-1",
      round: 1,
      submission_id: "s_1",
      submitted_at: "2026-10-25T15:04:05Z",
      outcome: "changes",
      document: { id: A.inspect.id, content_sha256: A.inspect.content_sha256 },
    });
    expect(session.read().entries[0]).toMatchObject({
      state: "changes",
      submission_id: "s_1",
    });
    // The producer writes what the closed schema allows, nothing more.
    expect(closedReview(rec), JSON.stringify(closedReview.errors)).toBe(true);
    expect(review.settled()).toBe(true);
  });

  test("a replayed submission id is acknowledged without a second record", async () => {
    const { review, key } = await opened();
    await review.record(A.inspect.id, "approve", key("s_1"));
    const again = await review.record(A.inspect.id, "approve", key("s_1"));
    expect(again.status).toBe(200);
    expect(review.records()).toHaveLength(1);
  });

  test("a closed round and a wrong round id are 409", async () => {
    const { review, key } = await opened();
    expect(
      (
        await review.record(A.inspect.id, "dismiss", {
          ...key("s_1"),
          round: "x",
        })
      ).status,
    ).toBe(409);
    await review.record(A.inspect.id, "approve", key("s_1"));
    expect(
      (await review.record(A.inspect.id, "dismiss", key("s_2"))).status,
    ).toBe(409);
  });

  test("a run that ends while docz reads the file refuses the submission", async () => {
    const { review, key, docz, session, entries } = await opened();
    const inspect = docz.inspect.bind(docz);
    docz.inspect = (...args) => {
      review.dismissRest();
      return inspect(...args);
    };
    const res = await review.record(A.inspect.id, "submit", {
      ...key("s_1"),
      ...changes,
    });
    expect(res).toEqual({
      status: 409,
      body: { error: "the round is closed" },
    });
    expect(review.records()).toEqual([]);
    expect(session.readRecord(entries[0]?.key ?? "", 1)).toBeNull();
  });

  test("the file edited, replaced, or deleted while open is stale and keeps the draft", async () => {
    const edited = docAs(A.inspect.id, A.inspect.path, "---\nedited\n");
    const replaced = docAs("DESIGN-0099", A.inspect.path);
    for (const [name, inspect] of [
      ["edited", { stdout: JSON.stringify(edited.inspect) }],
      ["replaced", { stdout: JSON.stringify(replaced.inspect) }],
      ["deleted", { code: 1, stderr: "Error: no document\n" }],
    ] as const) {
      const { review, key, fake, session, entries } = await opened();
      const draft = {
        questions: {},
        annotations: [],
        status: "Draft",
        findings: [],
      };
      review.saveDraft(A.inspect.id, { round: key("x").round, draft });
      fake.set({
        ...doczConfig([A]),
        [`inspect ${A.inspect.path}`]: [inspect],
      });
      const reply = await review.record(A.inspect.id, "approve", key("s_1"));
      expect(reply.status, name).toBe(409);
      expect(session.readDraft(entries[0]?.key ?? "", 1), name).toEqual(draft);
      expect(review.records(), name).toEqual([]);
    }
  });

  test("a choice, status, or finding the round did not offer is 400", async () => {
    const { review, key } = await opened();
    const bad = [
      { questions: [{ number: 1, choice: "z" }] },
      { questions: [{ number: 9, choice: "a" }] },
      { status: "Shipped" },
      { findings: [{ code: "made.up", line: 1, action: "fix" }] },
      { outcome: "maybe" },
      { annotations: [{ order: 1, kind: "shout" }] },
    ];
    for (const [i, over] of bad.entries()) {
      const reply = await review.record(A.inspect.id, "submit", {
        ...key(`s_${String(i)}`),
        ...changes,
        ...over,
      });
      expect(reply.status, JSON.stringify(over)).toBe(400);
    }
    expect(review.records()).toEqual([]);
  });

  test("two submissions at once are taken one at a time", async () => {
    const { review, key } = await opened();
    const replies = await Promise.all([
      review.record(A.inspect.id, "approve", key("s_1")),
      review.record(A.inspect.id, "dismiss", key("s_2")),
    ]);
    expect(replies.map((r) => r.status)).toEqual([200, 409]);
  });
});

describe("Review", () => {
  test("feedback covers only the documents that ended in changes", async () => {
    const B = docAs("DESIGN-0043", "docs/design/0043-b.md");
    const C = docAs("DESIGN-0044", "docs/design/0044-c.md");
    const { review, session } = await opened([A, B, C]);
    const docs = review.view().documents;
    const keyOf = (i: number, s: string) => ({
      round: docs[i]?.round,
      content_sha256: docs[i]?.inspect.content_sha256,
      submission_id: s,
    });
    await review.record(A.inspect.id, "approve", keyOf(0, "s_a"));
    await review.record(B.inspect.id, "submit", {
      ...keyOf(1, "s_b"),
      ...changes,
    });
    expect(review.settled()).toBe(false);
    const msg = review.feedback() ?? "";
    expect(msg).toContain("DESIGN-0043");
    expect(msg).not.toContain("DESIGN-0042");
    review.dismissRest();
    expect(session.read().entries.map((e) => e.state)).toEqual([
      "approved",
      "changes",
      "dismissed",
    ]);
    expect(review.records().map((r) => r.document.id)).toEqual([
      "DESIGN-0042",
      "DESIGN-0043",
    ]);
  });

  test("approval alone sends no feedback", async () => {
    const { review, key } = await opened();
    await review.record(A.inspect.id, "approve", key("s_1"));
    expect(review.feedback()).toBeNull();
  });

  test("the second round's page carries the first round's source", async () => {
    const first = await opened();
    await first.review.record(A.inspect.id, "submit", {
      ...first.key("s_1"),
      ...changes,
    });
    const next = docAs(
      A.inspect.id,
      A.inspect.path,
      `${A.inspect.content}\nmore\n`,
    );
    first.fake.set(doczConfig([next]));
    const { review } = await Review.open(first.session.read().entries, {
      session: first.session,
      docz: first.docz,
      reviewId: "sess-1",
      deadline: new Date(),
      maxRounds: 5,
    });
    const doc = review.view().documents[0];
    expect(doc?.round_number).toBe(2);
    expect(doc?.previous).toBe(A.inspect.content);
  });

  test("a draft for another round is refused and the saved one survives a restart", async () => {
    const { review, key, session, entries } = await opened();
    const draft = {
      questions: { 1: { choice: "a" } },
      annotations: [],
      status: "Draft",
      findings: [],
    };
    expect(review.saveDraft(A.inspect.id, { round: "old", draft }).status).toBe(
      409,
    );
    expect(
      review.saveDraft(A.inspect.id, { round: key("x").round, draft }).status,
    ).toBe(200);
    expect(session.readDraft(entries[0]?.key ?? "", 1)).toEqual(draft);
  });
});
