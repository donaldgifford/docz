import { buttonVariants } from "@donaldgifford/design-system/components/button";
import { IconSearch } from "@tabler/icons-react";
import { Link } from "react-router";

import { EmptyPanel } from "@/components/query-states";

/** The catch-all route: a path no route matches. */
export function Component() {
  return (
    <main className="px-5">
      <EmptyPanel
        icon={IconSearch}
        title="404 — nothing here"
        description="This path doesn't match any route."
        action={
          <Link
            to="/"
            className={buttonVariants({ variant: "outline", size: "sm" })}
          >
            Back to the directory
          </Link>
        }
      />
    </main>
  );
}
