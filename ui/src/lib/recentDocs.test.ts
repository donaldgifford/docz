import { describe, expect, it } from "vitest";

import { readRecentDocs, recordRecentDoc } from "@/lib/recentDocs";

const KEY = "docz:recent-docs";

function entry(n: number) {
  return {
    kind: "doc" as const,
    repo: "donaldgifford/docz-site",
    type: "design",
    docId: `DESIGN-${String(n).padStart(4, "0")}`,
    title: `Doc ${String(n)}`,
  };
}

function pageEntry(path: string) {
  return {
    kind: "page" as const,
    repo: "donaldgifford/docz-site",
    path,
    title: `Page ${path}`,
  };
}

describe("recentDocs", () => {
  it("records most-recent first and dedupes coordinates", () => {
    recordRecentDoc(entry(1));
    recordRecentDoc(entry(2));
    recordRecentDoc(entry(1)); // re-open moves it back to the front

    const docs = readRecentDocs();
    expect(
      docs.map((doc) => (doc.kind === "doc" ? doc.docId : doc.path)),
    ).toEqual(["DESIGN-0001", "DESIGN-0002"]);
  });

  it("round-trips page entries alongside docs", () => {
    recordRecentDoc(entry(1));
    recordRecentDoc(pageEntry("guides/local-dev.md"));
    recordRecentDoc(pageEntry("guides/local-dev.md")); // dedupes on path

    const docs = readRecentDocs();
    expect(docs).toEqual([pageEntry("guides/local-dev.md"), entry(1)]);
  });

  it("caps the list at 8, evicting the oldest", () => {
    for (let i = 1; i <= 10; i += 1) {
      recordRecentDoc(entry(i));
    }
    const docs = readRecentDocs();
    expect(docs).toHaveLength(8);
    expect(docs[0]).toMatchObject({ docId: "DESIGN-0010" });
    expect(docs.at(-1)).toMatchObject({ docId: "DESIGN-0003" });
  });

  it("resets the store on malformed payloads", () => {
    localStorage.setItem(KEY, "{not json");
    expect(readRecentDocs()).toEqual([]);
    expect(localStorage.getItem(KEY)).toBeNull();

    localStorage.setItem(KEY, JSON.stringify([{ repo: 42 }]));
    expect(readRecentDocs()).toEqual([]);
    expect(localStorage.getItem(KEY)).toBeNull();
  });

  it("resets the store on the pre-kind stored shape", () => {
    const preKind: Partial<ReturnType<typeof entry>> = { ...entry(1) };
    delete preKind.kind;
    localStorage.setItem(KEY, JSON.stringify([preKind]));
    expect(readRecentDocs()).toEqual([]);
    expect(localStorage.getItem(KEY)).toBeNull();
  });

  it("rejects path-hostile segments on read and write", () => {
    localStorage.setItem(
      KEY,
      JSON.stringify([{ ...entry(1), docId: "../../../etc" }]),
    );
    expect(readRecentDocs()).toEqual([]);

    recordRecentDoc({ ...entry(2), repo: "no-slash" });
    expect(readRecentDocs()).toEqual([]);
  });

  it("validates page paths per segment on read and write", () => {
    // Dot-only segments would router-resolve upward out of the route.
    recordRecentDoc(pageEntry("../secrets"));
    recordRecentDoc(pageEntry("guides/../../etc"));
    recordRecentDoc(pageEntry("guides//local-dev.md"));
    expect(readRecentDocs()).toEqual([]);

    localStorage.setItem(KEY, JSON.stringify([pageEntry("has space/x.md")]));
    expect(readRecentDocs()).toEqual([]);
    expect(localStorage.getItem(KEY)).toBeNull();
  });
});
