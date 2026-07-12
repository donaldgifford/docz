import { describe, expect, it } from "vitest";

import { getDoc, listRepos, listTypes } from "@/api/__generated__/docz-api";
import { DEMO_DOCS } from "@/mocks/fixtures";

describe("demo-org fixtures", () => {
  it("serves the curated repo list", async () => {
    const response = await listRepos();
    if (response.status !== 200) throw new Error("expected 200");
    expect(response.data.repos.map((r) => r.repo)).toEqual([
      "donaldgifford/docz-site",
      "donaldgifford/docz-api",
    ]);
  });

  it("serves this repo's real DESIGN-0001 with raw_md", async () => {
    const response = await getDoc(
      "donaldgifford",
      "docz-site",
      "design",
      "DESIGN-0001",
    );
    if (response.status !== 200) throw new Error("expected 200");
    const doc = response.data;
    expect(doc.title).toBe("docz-site: cross-repo docz reader and search UI");
    expect(doc.author).toBe("Donald Gifford");
    expect(doc.raw_md).toContain("<!--toc:start-->");
    expect(doc.raw_md).toContain("## ");
  });

  it("404s an unknown doc inside the demo org", async () => {
    await expect(
      getDoc("donaldgifford", "docz-site", "design", "DESIGN-9999"),
    ).rejects.toMatchObject({ status: 404 });
  });

  it("falls through to faker handlers outside the demo org", async () => {
    const response = await listTypes("someone-else", "another-repo");
    if (response.status !== 200) throw new Error("expected 200");
    // Faker shapes, not our fixtures — just prove the layer fell through
    // and produced a contract-valid body.
    expect(Array.isArray(response.data.types)).toBe(true);
  });

  it("every fixture doc has the frontmatter-derived identity fields", () => {
    for (const doc of DEMO_DOCS) {
      expect(doc.doc_id).toMatch(/^[A-Z]+-\d{4}$/);
      expect(doc.title).not.toBe("");
      expect(doc.status).not.toBe("");
      expect(doc.raw_md ?? "").not.toBe("");
    }
  });
});
