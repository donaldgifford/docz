import { describe, expect, it } from "vitest";

import { readRecentDocs, recordRecentDoc } from "@/lib/recentDocs";

const KEY = "docz:recent-docs";

function entry(n: number) {
  return {
    repo: "donaldgifford/docz-site",
    type: "design",
    docId: `DESIGN-${String(n).padStart(4, "0")}`,
    title: `Doc ${String(n)}`,
  };
}

describe("recentDocs", () => {
  it("records most-recent first and dedupes coordinates", () => {
    recordRecentDoc(entry(1));
    recordRecentDoc(entry(2));
    recordRecentDoc(entry(1)); // re-open moves it back to the front

    const docs = readRecentDocs();
    expect(docs.map((doc) => doc.docId)).toEqual([
      "DESIGN-0001",
      "DESIGN-0002",
    ]);
  });

  it("caps the list at 8, evicting the oldest", () => {
    for (let i = 1; i <= 10; i += 1) {
      recordRecentDoc(entry(i));
    }
    const docs = readRecentDocs();
    expect(docs).toHaveLength(8);
    expect(docs[0]?.docId).toBe("DESIGN-0010");
    expect(docs.at(-1)?.docId).toBe("DESIGN-0003");
  });

  it("resets the store on malformed payloads", () => {
    localStorage.setItem(KEY, "{not json");
    expect(readRecentDocs()).toEqual([]);
    expect(localStorage.getItem(KEY)).toBeNull();

    localStorage.setItem(KEY, JSON.stringify([{ repo: 42 }]));
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
});
