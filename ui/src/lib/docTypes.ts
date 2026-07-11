import type { DocType } from "@/api/__generated__/docz-api.schemas";

/*
 * `{type}` in URLs resolves by canonical name, id_prefix, or alias —
 * mirroring docz-api's path resolution (DESIGN-0001 "IA and routes").
 * Links are always GENERATED from the canonical name; this helper only
 * widens what we accept.
 */
export function resolveDocType(
  types: DocType[],
  slug: string,
): DocType | undefined {
  const lower = slug.toLowerCase();
  return types.find(
    (docType) =>
      docType.name.toLowerCase() === lower ||
      docType.id_prefix.toLowerCase() === lower ||
      docType.aliases.some((alias) => alias.toLowerCase() === lower),
  );
}

/**
 * Curated one-line blurbs for the standard docz types (mockup
 * TYPE_META); custom types fall back to a generic line.
 */
const TYPE_BLURBS: Record<string, string> = {
  rfc: "Immutable decision records — one document per accepted decision.",
  adr: "Small, focused records of a single architectural decision: the context, the decision, and its consequences.",
  design:
    "System and component designs — the shape of a solution before it is built.",
  impl: "Concrete steps to build a feature or system, with ordered tasks and milestones.",
  plan: "Higher-level planning and sequencing across multiple pieces of work.",
  investigation:
    "Explorations of a problem space — a menu of options to review, not a decision.",
  principle: "The values everything else instantiates. Changed only by RFC.",
  mandate:
    "Non-negotiable properties every system must satisfy, each enforced by tooling.",
  guide:
    "The paved road — choosing it needs no justification; leaving it needs an RFC.",
  policy:
    "Bindings that apply a framework to a scope, with the rationale for why.",
  framework:
    "Control sets, generated from fwsync HCL and mapped to external standards.",
};

export function typeBlurb(docType: DocType): string {
  return (
    TYPE_BLURBS[docType.name.toLowerCase()] ??
    `${docType.plural_label} for this repository.`
  );
}
