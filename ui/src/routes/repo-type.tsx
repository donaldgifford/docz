import { useParams } from "react-router";

export function Component() {
  const { owner, repo, type } = useParams();
  return (
    <main className="p-6">
      <h1 className="font-mono text-fg-secondary">
        {type} in {owner}/{repo}
      </h1>
      <p className="text-fg-tertiary">
        Placeholder — the type listing lands in Phase 3.
      </p>
    </main>
  );
}
