import { afterEach, describe, expect, it } from "vitest";

import {
  createReviewClient,
  newSubmissionId,
  parseEvent,
  ReviewApiError,
  StaleError,
  takeToken,
  type ReviewEvent,
  type Submission,
} from "@/review/api";
import { emptyDraft } from "@/review/draft";
import { FakeReviewServer } from "@/review/fake-server";
import { INSPECT, reviewDocument, TOKEN } from "@/review/testing";

function setup() {
  const server = new FakeReviewServer({
    token: TOKEN,
    documents: [reviewDocument()],
    now: () => new Date("2026-10-25T10:00:00.123Z"),
  });
  return { server, client: createReviewClient(TOKEN, server.fetch) };
}

const key = (over: Partial<Submission> = {}): Submission => ({
  round: "round-DESIGN-0042-1",
  content_sha256: INSPECT.content_sha256,
  submission_id: newSubmissionId(),
  outcome: "changes",
  questions: [{ number: 1, choice: "a" }],
  annotations: [],
  status: "Draft",
  findings: [],
  ...over,
});

describe("takeToken", () => {
  afterEach(() => {
    window.history.replaceState(null, "", "/");
  });

  it("reads the token and removes it from the location", () => {
    window.history.replaceState(null, "", `/?t=${TOKEN}&x=1#top`);
    expect(takeToken()).toBe(TOKEN);
    expect(window.location.search).toBe("?x=1");
    expect(window.location.hash).toBe("#top");
  });

  it("returns null without one", () => {
    window.history.replaceState(null, "", "/");
    expect(takeToken()).toBeNull();
  });
});

describe("the review client against the fake server", () => {
  it("sends the bearer token and loads the session", async () => {
    const { client } = setup();
    const session = await client.session();
    expect(session.documents.map((d) => d.key)).toEqual(["DESIGN-0042"]);
  });

  it("is refused without the right token", async () => {
    const { server } = setup();
    const wrong = createReviewClient("nope", server.fetch);
    await expect(wrong.session()).rejects.toMatchObject({ status: 401 });
  });

  it("records a submission once, whatever the retries", async () => {
    const { server, client } = setup();
    const body = key();
    expect(await client.submit("DESIGN-0042", body)).toEqual({
      outcome: "changes",
      state: "changes",
    });
    await client.submit("DESIGN-0042", body);
    expect(server.records).toHaveLength(1);
    expect(server.records[0]).toMatchObject({
      round: 1,
      submitted_at: "2026-10-25T10:00:00Z",
      document: { id: "DESIGN-0042", content_sha256: INSPECT.content_sha256 },
    });
  });

  it("refuses a second submission to a closed round", async () => {
    const { client } = setup();
    await client.submit("DESIGN-0042", key());
    await expect(client.submit("DESIGN-0042", key())).rejects.toBeInstanceOf(
      StaleError,
    );
  });

  it("calls a submission against changed bytes stale", async () => {
    const { server, client } = setup();
    server.edit("DESIGN-0042", { ...INSPECT, content_sha256: "b".repeat(64) });
    await expect(client.approve("DESIGN-0042", key())).rejects.toBeInstanceOf(
      StaleError,
    );
    expect(server.records).toEqual([]);
  });

  it.each([
    [
      "an option the question lacks",
      { questions: [{ number: 1, choice: "z" }] },
    ],
    [
      "a resolved question's letter",
      { questions: [{ number: 9, choice: "a" }] },
    ],
    ["a status the type lacks", { status: "Shipped" }],
    [
      "a finding the round did not report",
      { findings: [{ code: "made.up", line: 1, action: "fix" as const }] },
    ],
  ])("refuses %s with 400", async (_name, over) => {
    const { client } = setup();
    const err: unknown = await client
      .submit("DESIGN-0042", key(over))
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ReviewApiError);
    expect((err as ReviewApiError).status).toBe(400);
  });

  it("approves and dismisses with the document's own status", async () => {
    const { server, client } = setup();
    expect(await client.approve("DESIGN-0042", key())).toEqual({
      outcome: "approved",
      state: "approved",
    });
    expect(server.records[0]).toMatchObject({ status: "Draft", questions: [] });
  });

  it("autosaves a draft into the session", async () => {
    const { client } = setup();
    const draft = emptyDraft("Draft");
    draft.questions[2] = { choice: "b" };
    await client.saveDraft("DESIGN-0042", "round-DESIGN-0042-1", draft);
    const session = await client.session();
    expect(session.documents[0]?.draft).toEqual(draft);
  });

  it("streams events", async () => {
    const { server, client } = setup();
    const got: ReviewEvent[] = [];
    const abort = new AbortController();
    const reading = client.events((e) => got.push(e), abort.signal);
    await new Promise((r) => setTimeout(r, 0));
    server.nextRound("DESIGN-0042", INSPECT);
    await client.done();
    await new Promise((r) => setTimeout(r, 0));
    abort.abort();
    await reading.catch(() => undefined);
    expect(got).toEqual([
      { type: "updated" },
      { type: "closed", reason: "done" },
    ]);
  });
});

describe("parseEvent", () => {
  it("ignores comments and unknown events", () => {
    expect(parseEvent(": keepalive")).toBeNull();
    expect(parseEvent("event: other\ndata: x")).toBeNull();
    expect(parseEvent("event: closed\ndata: deadline")).toEqual({
      type: "closed",
      reason: "deadline",
    });
  });
});
