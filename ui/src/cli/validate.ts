/*
 * Runtime validation of the docz CLI's JSON output (DESIGN-0024 §3).
 *
 * Generated types say what docz-review expects; they check nothing at run
 * time. Every CLI output therefore goes through one of the parse functions
 * here: a size limit, JSON.parse, then ajv over the schema's OPEN variant
 * (additionalProperties removed by scripts/gen-cli.ts), so a newer docz
 * that adds a field still validates while one that drops or retypes a field
 * is refused. Errors are not findings: a ContractError means the output
 * could not be trusted, and nothing turns one into an empty report.
 */
import {
  Ajv2020,
  type ErrorObject,
  type ValidateFunction,
} from "ajv/dist/2020";

import type { DoczCreate } from "@/cli/__generated__/create";
import createSchema from "@/cli/__generated__/create.open.json";
import type { DoczInspect } from "@/cli/__generated__/inspect";
import inspectSchema from "@/cli/__generated__/inspect.open.json";
import type { DoczReview } from "@/cli/__generated__/review";
import reviewSchema from "@/cli/__generated__/review.open.json";
import type { DoczValidate } from "@/cli/__generated__/validate";
import validateSchema from "@/cli/__generated__/validate.open.json";

/** The largest CLI output docz-review will parse, in bytes. */
export const MAX_OUTPUT_BYTES = 16 * 1024 * 1024;

/**
 * The contract this build of docz-review speaks: the schema_version it
 * reads, and the first docz release that produced it. A docz at or above
 * that release with the same schema_version interoperates.
 */
export const SUPPORTED = { schemaVersion: 1, minDocz: "2.2.0" } as const;

export type SchemaName = "inspect" | "validate" | "create" | "review";

/** ContractError reports output that does not match its schema. */
export class ContractError extends Error {
  readonly schema: SchemaName;
  readonly errors: readonly ErrorObject[];

  constructor(
    schema: SchemaName,
    message: string,
    errors: readonly ErrorObject[] = [],
  ) {
    super(`docz ${schema} output: ${message}`);
    this.name = "ContractError";
    this.schema = schema;
    this.errors = errors;
  }
}

/** VersionError reports a docz whose contract this build does not speak. */
export class VersionError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "VersionError";
  }
}

// One compiler, each schema compiled once on first use. $version is the
// schemas' own annotation; formats are asserted by the Go producer test,
// so here they are annotations too.
const ajv = new Ajv2020({ allErrors: true, validateFormats: false });
ajv.addKeyword("$version");

const sources: Record<SchemaName, object> = {
  inspect: inspectSchema,
  validate: validateSchema,
  create: createSchema,
  review: reviewSchema,
};

const compiled = new Map<SchemaName, ValidateFunction>();

function validator(name: SchemaName): ValidateFunction {
  let fn = compiled.get(name);
  if (fn === undefined) {
    fn = ajv.compile(sources[name]);
    compiled.set(name, fn);
  }
  return fn;
}

function parse(name: SchemaName, raw: string): unknown {
  if (new TextEncoder().encode(raw).length > MAX_OUTPUT_BYTES) {
    throw new ContractError(
      name,
      `larger than ${String(MAX_OUTPUT_BYTES)} bytes`,
    );
  }

  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch (err) {
    throw new ContractError(name, `not JSON: ${(err as Error).message}`);
  }

  const fn = validator(name);
  if (!fn(value)) {
    const errors = fn.errors ?? [];
    throw new ContractError(name, ajv.errorsText(errors), errors);
  }

  return value;
}

/** parseInspect validates `docz inspect --format json` output. */
export function parseInspect(raw: string): DoczInspect {
  const out = parse("inspect", raw) as DoczInspect;
  checkSchemaVersion(out.schema_version, out.docz_version);
  return out;
}

/** parseValidate validates `docz validate --format json` output. */
export function parseValidate(raw: string): DoczValidate {
  const out = parse("validate", raw) as DoczValidate;
  checkSchemaVersion(out.schema_version);
  return out;
}

/** parseCreate validates `docz create --format json` output. */
export function parseCreate(raw: string): DoczCreate {
  const out = parse("create", raw) as DoczCreate;
  checkSchemaVersion(out.schema_version);
  return out;
}

/** parseReview validates a review record. */
export function parseReview(raw: string): DoczReview {
  const out = parse("review", raw) as DoczReview;
  checkSchemaVersion(out.schema_version);
  return out;
}

/**
 * checkSchemaVersion refuses a docz whose contract this build does not
 * speak, naming both sides. A development build is accepted at any
 * version: "dev" carries no release number, and a build from a checkout
 * (`v2.1.3-50-g6115270`, git describe) is commits past its tag, so the
 * tag says nothing about whether it has the contract. The schema_version
 * check still applies to both.
 */
export function checkSchemaVersion(
  schemaVersion: number,
  doczVersion?: string,
): void {
  if (schemaVersion !== SUPPORTED.schemaVersion) {
    throw new VersionError(
      `docz output has schema_version ${String(schemaVersion)}${doczVersion === undefined ? "" : ` (docz ${doczVersion})`}; ` +
        `this docz-review reads schema_version ${String(SUPPORTED.schemaVersion)} from docz ${SUPPORTED.minDocz} or later`,
    );
  }

  if (
    doczVersion === undefined ||
    doczVersion === "dev" ||
    DESCRIBE_BUILD.test(doczVersion)
  ) {
    return;
  }

  if (compareVersions(doczVersion, SUPPORTED.minDocz) < 0) {
    throw new VersionError(
      `docz ${doczVersion} predates the contract; this docz-review needs docz ${SUPPORTED.minDocz} or later ` +
        `(schema_version ${String(SUPPORTED.schemaVersion)})`,
    );
  }
}

// git describe's "-<commits>-g<hash>" suffix: a build from a checkout.
const DESCRIBE_BUILD = /-\d+-g[0-9a-f]+(?:-dirty)?$/;

/** compareVersions orders two release versions by major, minor, patch. */
export function compareVersions(a: string, b: string): number {
  const parts = (v: string): number[] =>
    (v.replace(/^v/, "").split(/[-+]/)[0] ?? "")
      .split(".")
      .map((n) => Number.parseInt(n, 10) || 0);

  const pa = parts(a);
  const pb = parts(b);

  for (let i = 0; i < 3; i++) {
    const d = (pa[i] ?? 0) - (pb[i] ?? 0);
    if (d !== 0) {
      return d;
    }
  }
  return 0;
}
