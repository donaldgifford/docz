import { Link } from "react-router";

export function Component() {
  return (
    <main className="p-6">
      <h1 className="font-mono text-fg-secondary">404 — nothing here</h1>
      <p className="text-fg-tertiary">
        This path doesn&apos;t match any route.{" "}
        <Link to="/" className="text-accent">
          Back to the directory
        </Link>
        .
      </p>
    </main>
  );
}
