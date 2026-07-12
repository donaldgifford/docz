import { useParams } from "react-router";

export function Component() {
  const { owner, repo } = useParams();
  return (
    <main className="p-6">
      <h1 className="font-mono text-fg-secondary">
        Repo home — {owner}/{repo}
      </h1>
      <p className="text-fg-tertiary">
        Placeholder — the repo home page lands in Phase 3.
      </p>
    </main>
  );
}
