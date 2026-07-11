import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { renderMarkdown } from "@/markdown/processor";
import { useRenderedMarkdown } from "@/markdown/useRenderedMarkdown";

import type { Document } from "@/api/__generated__/docz-api.schemas";

vi.mock("@/markdown/processor", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/markdown/processor")>();
  return { ...actual, renderMarkdown: vi.fn(actual.renderMarkdown) };
});

const renderMarkdownMock = vi.mocked(renderMarkdown);

function makeDoc(overrides: Partial<Document> = {}): Document {
  return {
    repo: "acme/docs",
    doc_id: "DESIGN-0001",
    type: "design",
    title: "A design",
    status: "Draft",
    author: "someone",
    created: "2026-01-01",
    path: "docs/design/0001-a-design.md",
    git_sha: "0123456789abcdef",
    content_hash: "hash-a",
    updated_at: "2026-07-01T00:00:00Z",
    raw_md: "# A design\n\n## Section\n\nBody.",
    ...overrides,
  };
}

describe("useRenderedMarkdown", () => {
  it("runs the pipeline once per (doc_id, content_hash)", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    function wrapper({ children }: { children: ReactNode }) {
      return (
        <QueryClientProvider client={queryClient}>
          {children}
        </QueryClientProvider>
      );
    }
    const doc = makeDoc();

    const first = renderHook(() => useRenderedMarkdown(doc), { wrapper });
    await waitFor(() => {
      expect(first.result.current.isSuccess).toBe(true);
    });
    first.unmount();

    // Same identity + content — served from cache, no second run.
    const second = renderHook(() => useRenderedMarkdown(makeDoc()), {
      wrapper,
    });
    await waitFor(() => {
      expect(second.result.current.isSuccess).toBe(true);
    });
    expect(renderMarkdownMock).toHaveBeenCalledTimes(1);
    second.unmount();

    // New content hash — the doc changed, so the pipeline reruns.
    const third = renderHook(
      () =>
        useRenderedMarkdown(
          makeDoc({ content_hash: "hash-b", raw_md: "# Changed" }),
        ),
      { wrapper },
    );
    await waitFor(() => {
      expect(third.result.current.isSuccess).toBe(true);
    });
    expect(renderMarkdownMock).toHaveBeenCalledTimes(2);
  });

  it("stays idle without raw_md (list responses have none)", () => {
    const queryClient = new QueryClient();
    function wrapper({ children }: { children: ReactNode }) {
      return (
        <QueryClientProvider client={queryClient}>
          {children}
        </QueryClientProvider>
      );
    }

    const { result } = renderHook(
      () => useRenderedMarkdown(makeDoc({ raw_md: undefined })),
      { wrapper },
    );
    expect(result.current.fetchStatus).toBe("idle");
    expect(result.current.data).toBeUndefined();
  });
});
