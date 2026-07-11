/*
 * Curated "demo org" fixtures: real docz markdown served through MSW so
 * rendering truth (frontmatter, toc markers, tables, fenced code) is
 * exercised alongside the faker handlers' shape coverage. Layered FIRST
 * in both the test server and the dev-msw worker — resolvers fall
 * through (return undefined) for anything outside the demo org, letting
 * orval's faker handlers answer.
 *
 * docz-site docs are ?raw imports of the actual files (always current);
 * docz-api docs are snapshots under src/mocks/content/ (the originals
 * live outside this repo).
 */
import { http, HttpResponse } from "msw";

import doczSiteDesign0001 from "../../docs/design/0001-docz-site-cross-repo-docz-reader-and-search-ui.md?raw";
import doczSiteImpl0001 from "../../docs/impl/0001-docz-site-mvp-phased-build-of-the-reader-directory-and-repo.md?raw";
import doczApiDesign0001 from "./content/docz-api-design-0001.md?raw";
import doczApiDesign0002 from "./content/docz-api-design-0002.md?raw";

import type {
  DocType,
  Document,
  RepoDetail,
  RepoSummary,
  SearchHit,
} from "@/api/__generated__/docz-api.schemas";

function frontmatterField(raw: string, key: string): string {
  const fm = /^---\r?\n([\s\S]*?)\r?\n---/.exec(raw)?.[1] ?? "";
  const line = new RegExp(`^${key}:\\s*(.*)$`, "m").exec(fm)?.[1] ?? "";
  return line.trim().replace(/^"(.*)"$/, "$1");
}

interface FixtureDocInput {
  repo: string;
  type: string;
  path: string;
  raw: string;
  updatedAt: string;
}

function makeDoc(input: FixtureDocInput): Document {
  const docId = frontmatterField(input.raw, "id");
  return {
    repo: input.repo,
    doc_id: docId,
    type: input.type,
    title: frontmatterField(input.raw, "title"),
    status: frontmatterField(input.raw, "status"),
    author: frontmatterField(input.raw, "author"),
    created: frontmatterField(input.raw, "created"),
    path: input.path,
    git_sha: `fixture-sha-${docId.toLowerCase()}`,
    content_hash: `fixture-hash-${docId.toLowerCase()}`,
    updated_at: input.updatedAt,
    raw_md: input.raw,
  };
}

const DESIGN_STATUSES = [
  "Draft",
  "In Review",
  "Approved",
  "Implemented",
  "Abandoned",
];
const IMPL_STATUSES = [
  "Draft",
  "In Progress",
  "Completed",
  "Paused",
  "Cancelled",
];
const INVESTIGATION_STATUSES = [
  "Open",
  "In Progress",
  "Concluded",
  "Inconclusive",
  "Abandoned",
];

// Real type declarations from the repos' .docz.yaml files.
const DEMO_TYPES: Record<string, DocType[]> = {
  "donaldgifford/docz-site": [
    {
      name: "design",
      dir: "design",
      id_prefix: "DESIGN",
      plural_label: "Designs",
      statuses: DESIGN_STATUSES,
      aliases: [],
    },
    {
      name: "impl",
      dir: "impl",
      id_prefix: "IMPL",
      plural_label: "Implementation Plans",
      statuses: IMPL_STATUSES,
      aliases: [],
    },
    {
      name: "investigation",
      dir: "investigation",
      id_prefix: "INV",
      plural_label: "Investigations",
      statuses: INVESTIGATION_STATUSES,
      aliases: ["inv"],
    },
  ],
  "donaldgifford/docz-api": [
    {
      name: "design",
      dir: "design",
      id_prefix: "DESIGN",
      plural_label: "Designs",
      statuses: DESIGN_STATUSES,
      aliases: [],
    },
    {
      name: "rfc",
      dir: "rfc",
      id_prefix: "RFC",
      plural_label: "RFCs",
      statuses: ["Draft", "Proposed", "Accepted", "Rejected", "Superseded"],
      aliases: [],
    },
  ],
};

export const DEMO_DOCS: Document[] = [
  makeDoc({
    repo: "donaldgifford/docz-site",
    type: "design",
    path: "docs/design/0001-docz-site-cross-repo-docz-reader-and-search-ui.md",
    raw: doczSiteDesign0001,
    updatedAt: "2026-07-10T18:00:00Z",
  }),
  makeDoc({
    repo: "donaldgifford/docz-site",
    type: "impl",
    path: "docs/impl/0001-docz-site-mvp-phased-build-of-the-reader-directory-and-repo.md",
    raw: doczSiteImpl0001,
    updatedAt: "2026-07-11T09:00:00Z",
  }),
  makeDoc({
    repo: "donaldgifford/docz-api",
    type: "design",
    path: "docs/design/0001-docz-api-cross-repo-docz-registry-and-ingestion-service.md",
    raw: doczApiDesign0001,
    updatedAt: "2026-07-02T12:00:00Z",
  }),
  makeDoc({
    repo: "donaldgifford/docz-api",
    type: "design",
    path: "docs/design/0002-openapi-contract-for-docz-api-and-the-docz-site.md",
    raw: doczApiDesign0002,
    updatedAt: "2026-07-06T15:30:00Z",
  }),
];

const DEMO_REPOS: RepoSummary[] = Object.keys(DEMO_TYPES).map((repo) => ({
  repo,
  default_branch: "main",
  docs_dir: "docs",
  last_synced_sha: `fixture-head-${repo.split("/")[1] ?? repo}`,
}));

function repoKey(owner: string, name: string): string {
  return `${owner}/${name}`;
}

/** Strip raw_md — only the single-document endpoint carries it. */
function summary(doc: Document): Document {
  const { raw_md, ...rest } = doc;
  void raw_md;
  return rest;
}

function snippetFor(doc: Document, q: string): string {
  const body = (doc.raw_md ?? "").replace(/[#>`|-]/g, " ");
  const lower = body.toLowerCase();
  const at = q === "" ? -1 : lower.indexOf(q.toLowerCase());
  if (at === -1) {
    return body.slice(0, 140).trim();
  }
  const start = Math.max(0, at - 60);
  const before = body.slice(start, at);
  const match = body.slice(at, at + q.length);
  const after = body.slice(at + q.length, at + q.length + 80);
  return `${before}<em>${match}</em>${after}`.trim();
}

type Params = Record<string, string | readonly string[] | undefined>;

function str(value: Params[string]): string {
  return typeof value === "string" ? value : "";
}

function intParam(url: URL, key: string, fallback: number): number {
  const raw = url.searchParams.get(key);
  if (raw === null) {
    return fallback;
  }
  const value = Number.parseInt(raw, 10);
  return Number.isFinite(value) && value >= 0 ? value : fallback;
}

export const demoOrgHandlers = [
  http.get("*/api/v1/repos", () => HttpResponse.json({ repos: DEMO_REPOS })),

  http.get("*/api/v1/repos/:owner/:name", ({ params }) => {
    const key = repoKey(str(params.owner), str(params.name));
    const types = DEMO_TYPES[key];
    if (types === undefined) {
      return undefined; // outside the demo org — faker answers
    }
    const detail: RepoDetail = {
      repo: key,
      default_branch: "main",
      docs_dir: "docs",
      last_synced_sha: `fixture-head-${str(params.name)}`,
      config_snapshot: { docs_dir: "docs" },
      types,
    };
    return HttpResponse.json(detail);
  }),

  http.get("*/api/v1/repos/:owner/:name/types", ({ params }) => {
    const types = DEMO_TYPES[repoKey(str(params.owner), str(params.name))];
    if (types === undefined) {
      return undefined;
    }
    return HttpResponse.json({ types });
  }),

  http.get("*/api/v1/repos/:owner/:name/types/:type/docs", ({ params }) => {
    const key = repoKey(str(params.owner), str(params.name));
    if (DEMO_TYPES[key] === undefined) {
      return undefined;
    }
    const docs = DEMO_DOCS.filter(
      (doc) => doc.repo === key && doc.type === str(params.type),
    ).map(summary);
    return HttpResponse.json({ docs });
  }),

  http.get(
    "*/api/v1/repos/:owner/:name/types/:type/docs/:docId",
    ({ params }) => {
      const key = repoKey(str(params.owner), str(params.name));
      if (DEMO_TYPES[key] === undefined) {
        return undefined;
      }
      const doc = DEMO_DOCS.find(
        (candidate) =>
          candidate.repo === key &&
          candidate.type === str(params.type) &&
          candidate.doc_id.toLowerCase() === str(params.docId).toLowerCase(),
      );
      if (doc === undefined) {
        return HttpResponse.json({ error: "doc not found" }, { status: 404 });
      }
      return HttpResponse.json(doc);
    },
  ),

  http.get("*/api/v1/search", ({ request }) => {
    const url = new URL(request.url);
    const q = url.searchParams.get("q") ?? "";
    const repo = url.searchParams.get("repo");
    const type = url.searchParams.get("type");
    const status = url.searchParams.get("status");
    const author = url.searchParams.get("author");

    const matches = DEMO_DOCS.filter((doc) => {
      if (repo !== null && doc.repo !== repo) return false;
      if (type !== null && doc.type !== type) return false;
      if (status !== null && doc.status !== status) return false;
      if (author !== null && doc.author !== author) return false;
      if (q === "") return true;
      const haystack = `${doc.title}\n${doc.raw_md ?? ""}`.toLowerCase();
      return haystack.includes(q.toLowerCase());
    });

    // Facets and the estimated total cover the whole filtered set (as
    // Meilisearch does); only `hits` is the offset/limit window.
    const offset = intParam(url, "offset", 0);
    const limit = intParam(url, "limit", 20);
    const hits: SearchHit[] = matches
      .slice(offset, offset + limit)
      .map((doc) => ({
        repo: doc.repo,
        doc_id: doc.doc_id,
        type: doc.type,
        title: doc.title,
        status: doc.status,
        author: doc.author,
        snippet: snippetFor(doc, q),
      }));

    const facet = (key: (doc: Document) => string) =>
      Object.fromEntries(
        Object.entries(
          matches.reduce<Record<string, number>>((acc, doc) => {
            acc[key(doc)] = (acc[key(doc)] ?? 0) + 1;
            return acc;
          }, {}),
        ),
      );

    return HttpResponse.json({
      query: q,
      estimated_total_hits: hits.length,
      hits,
      facets: {
        repo: facet((doc) => doc.repo),
        type: facet((doc) => doc.type),
        status: facet((doc) => doc.status),
        author: facet((doc) => doc.author),
      },
    });
  }),
];
