import { Disclosure } from "@donaldgifford/design-system/blocks/disclosure";
import { Outline } from "@donaldgifford/design-system/blocks/outline";
import {
  Stepper,
  type Step,
  type StepStatus,
} from "@donaldgifford/design-system/blocks/stepper";
import { useActiveHeading } from "@donaldgifford/design-system/hooks/use-active-heading";

import { useListTypes } from "@/api/__generated__/docz-api";
import { arr } from "@/lib/wire";

import type { TocEntry } from "@/markdown/processor";

export type DocFormat = "html" | "md";

/*
 * The reader's side parts (DESIGN-0028 §7), each the design system's
 * block over docz's data: the outline of a page docz renders itself,
 * and a document's place in its type's lifecycle.
 */

/**
 * The band of the viewport a heading is being read in. The top margin
 * clears the sticky top bar, and the bottom one keeps a heading current
 * until the next climbs into the top slice.
 */
const READING_BAND = "-80px 0px -70% 0px";

/**
 * "On this page": the headings of a page as the design system's
 * Outline, with the one being read marked by its scroll spy. The spy
 * finds each heading by its id, so this follows whatever rendered them:
 * docz's own markdown render inside a DocSurface. It is undefined until
 * a heading reaches the band, and wherever IntersectionObserver is
 * missing; no row is marked then. Nothing is drawn for a page with no
 * headings.
 */
export function TocList({
  toc,
  label,
}: {
  toc: readonly TocEntry[];
  /** The heading over the list, and the landmark's name. */
  label?: string;
}) {
  const activeId = useActiveHeading(
    toc.map((entry) => entry.id),
    { rootMargin: READING_BAND },
  );
  return <Outline items={toc} activeId={activeId} label={label} />;
}

/** Where each of a type's statuses sits against the document's own. */
function lifecycleSteps(
  stages: readonly string[],
  currentStatus: string,
): Step[] {
  const current = stages.findIndex(
    (status) => status.toLowerCase() === currentStatus.toLowerCase(),
  );
  return stages.map((stage, index) => {
    // A status the type does not declare sits nowhere on the line, so
    // no stop is reached.
    let status: StepStatus = "upcoming";
    if (current !== -1 && index < current) {
      status = "done";
    } else if (index === current) {
      status = "current";
    }
    return { id: stage, label: stage, status };
  });
}

/**
 * Position-only lifecycle: the type's `statuses` from `.docz.yaml` (via
 * listTypes, cached per repo) as a vertical Stepper, with the doc's
 * status as the current step. No dates — docz-api doesn't expose
 * lifecycle timestamps yet (DESIGN-0001 additive ask). Renders nothing
 * when the type is unknown or declares no statuses — including its
 * disclosure, so an unknown type never leaves an empty box. It is
 * closed by default under the reader's metadata (IMPL-0002 Phase 5,
 * OQ-4a), and shows the status in force while closed.
 */
export function LifecycleRail({
  owner,
  name,
  typeName,
  currentStatus,
}: {
  owner: string;
  name: string;
  typeName: string;
  currentStatus: string;
}) {
  const typesQuery = useListTypes(owner, name, {
    // Type definitions change on .docz.yaml edits, not per navigation.
    query: { staleTime: 5 * 60_000 },
  });
  const types =
    typesQuery.data?.status === 200 ? typesQuery.data.data.types : undefined;
  const docType = arr(types).find(
    (t) => t.name === typeName || arr(t.aliases).includes(typeName),
  );
  const stages = arr(docType?.statuses);
  if (docType === undefined || stages.length === 0) {
    return null;
  }

  return (
    <Disclosure
      className="mb-6"
      label="Lifecycle"
      value={currentStatus === "" ? "unset" : currentStatus}
    >
      <Stepper
        orientation="vertical"
        size="sm"
        steps={lifecycleSteps(stages, currentStatus)}
      />
    </Disclosure>
  );
}
