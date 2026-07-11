import { useParams } from "react-router";

import { useGetDoc } from "@/api/__generated__/docz-api";
import { NotFoundError, SessionRequiredError } from "@/api/fetcher";
import {
  ErrorPanel,
  NotFoundPanel,
  SessionRequiredPanel,
} from "@/components/query-states";
import { useRenderedMarkdown } from "@/markdown/useRenderedMarkdown";

function ArticleSkeleton() {
  return (
    <div
      aria-hidden
      data-testid="doc-skeleton"
      className="mx-auto max-w-3xl animate-pulse px-6 py-10"
    >
      <div className="h-3 w-40 bg-bg-elevated" />
      <div className="mt-6 h-8 w-4/5 bg-bg-elevated" />
      <div className="mt-3 h-3 w-56 bg-bg-elevated" />
      <div className="mt-10 space-y-3">
        {Array.from({ length: 8 }, (_, i) => (
          <div
            key={i}
            className="h-3 bg-bg-raised"
            style={{ width: `${String(100 - (i % 4) * 9)}%` }}
          />
        ))}
      </div>
    </div>
  );
}

export function Component() {
  const { owner, repo, type, docId } = useParams();
  if (
    owner === undefined ||
    repo === undefined ||
    type === undefined ||
    docId === undefined
  ) {
    // The route pattern guarantees these; reaching here is a router bug.
    throw new Error("doc route rendered without its path params");
  }

  const docQuery = useGetDoc(owner, repo, type, docId);
  const doc =
    docQuery.data?.status === 200 ? docQuery.data.data : undefined;
  const rendered = useRenderedMarkdown(doc);

  if (docQuery.error instanceof SessionRequiredError) {
    return <SessionRequiredPanel />;
  }
  if (docQuery.error instanceof NotFoundError) {
    return <NotFoundPanel />;
  }
  if (docQuery.isError || rendered.isError) {
    const error = docQuery.error ?? rendered.error;
    return (
      <ErrorPanel
        message={error instanceof Error ? error.message : "Request failed"}
        onRetry={() => {
          void (docQuery.isError ? docQuery.refetch() : rendered.refetch());
        }}
      />
    );
  }
  if (doc === undefined || rendered.data === undefined) {
    return <ArticleSkeleton />;
  }

  return (
    <main className="mx-auto max-w-3xl px-6 py-10">
      <article className="doc-prose">
        <h1 className="text-fg-primary">{doc.title}</h1>
        {rendered.data.content}
      </article>
    </main>
  );
}
