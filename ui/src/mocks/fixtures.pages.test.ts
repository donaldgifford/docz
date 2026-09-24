import { describe, expect, it } from "vitest";

import { getRepoPage, listRepoPages } from "@/api/__generated__/docz-api";
import { DEMO_PAGES } from "@/mocks/fixtures";

/*
 * Realism pins for the pages fixtures (IMPL-0005 Phase 1): the demo
 * set must keep covering every published-path shape the upstream
 * mapping produces, and the resolvers must answer through the
 * generated client — both the one-segment percent-encoded spelling
 * the site sends and the literal-slash one docz-api also routes.
 */

const SITE_PAGES = DEMO_PAGES["donaldgifford/docz-site"] ?? [];

describe("pages fixture shape coverage", () => {
  it("covers directory, nested file, and additional_docs paths", () => {
    const paths = SITE_PAGES.map((page) => page.path);
    // Directory page: extensionless (a docz type dir's README).
    expect(paths).toContain("design");
    // Nested file page: keeps its extension.
    expect(paths).toContain("guides/local-dev.md");
    // additional_docs root file: repo-relative.
    expect(paths).toContain("README.md");
  });

  it("orders the list by path and strips raw_md from summaries", async () => {
    const response = await listRepoPages("donaldgifford", "docz-site");
    expect(response.status).toBe(200);
    if (response.status !== 200) return;
    const paths = response.data.pages.map((page) => page.path);
    expect(paths).toEqual([...paths].sort());
    expect(paths).toHaveLength(SITE_PAGES.length);
    for (const summary of response.data.pages) {
      expect(summary).not.toHaveProperty("raw_md");
      expect(summary.title).not.toBe("");
      expect(summary.git_sha).not.toBe("");
    }
  });

  it("a non-opted repo lists 200 with an empty set, never 404", async () => {
    const response = await listRepoPages("donaldgifford", "docz-api");
    expect(response.status).toBe(200);
    if (response.status !== 200) return;
    expect(response.data.pages).toEqual([]);
  });

  it("serves a nested page via the percent-encoded spelling", async () => {
    const response = await getRepoPage(
      "donaldgifford",
      "docz-site",
      encodeURIComponent("guides/local-dev.md"),
    );
    expect(response.status).toBe(200);
    if (response.status !== 200) return;
    expect(response.data.path).toBe("guides/local-dev.md");
    expect(response.data.raw_md).toContain("Local development");
  });

  it("serves the literal-slash spelling identically", async () => {
    const response = await getRepoPage(
      "donaldgifford",
      "docz-site",
      "guides/local-dev.md",
    );
    expect(response.status).toBe(200);
    if (response.status !== 200) return;
    expect(response.data.git_sha).toBe("fixture-page-sha-guides-local-dev");
  });

  it("404s a demo-org miss with the error envelope", async () => {
    await expect(
      getRepoPage("donaldgifford", "docz-site", "nope.md"),
    ).rejects.toMatchObject({ status: 404, message: "page not found" });
  });
});
