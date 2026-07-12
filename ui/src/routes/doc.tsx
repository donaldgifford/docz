import { useParams } from "react-router";

export function Component() {
  const { owner, repo, type, docId } = useParams();
  return (
    <main className="p-6">
      <h1 className="font-mono text-fg-secondary">
        {owner}/{repo} · {type} · {docId}
      </h1>
      <p className="text-fg-tertiary">
        Placeholder — the doc reader lands in Phase 1.
      </p>
    </main>
  );
}
