import {
  isAlias,
  isMap,
  isScalar,
  isSeq,
  LineCounter,
  parseAllDocuments,
  type Document,
  type Node as YamlNode,
} from "yaml";

import type { WidgetName } from "@/markdown/widgets/fences";

/*
 * The docz-flow, docz-stepper, and docz-scorecard validator, ported by
 * hand from pkg/export/html/widgets.go (IMPL-0030 Phase 5, Open
 * Question 9). It decodes a block the way yaml.v3 with KnownFields does
 * — a string field takes any scalar's text, an int field truncates a
 * float, a null is the zero value, and a mismatch names the YAML tag,
 * the value, and the Go type unless that type is one of the package's
 * own — then runs the same field checks with the same wording.
 * pkg/export/html/testdata/widgets holds both sides to the same errors
 * at the same lines and the same JSON. The one difference the corpus
 * allows is a block the parser itself refuses: that message is each
 * library's own, so only its line is held.
 */

export type { WidgetName };

export interface FlowStage {
  name: string;
  does: string;
  note?: string;
}

export interface FlowData {
  title?: string;
  stages: FlowStage[];
  gates?: number[];
  label?: string;
  legend?: string;
  alert?: string;
}

export interface StepperStep {
  key: string;
  name: string;
  title?: string;
  does: string;
}

export interface StepperScenario {
  name: string;
  fails_at?: string;
  fail_label?: string;
  notes?: Record<string, string>;
}

export interface StepperData {
  title?: string;
  steps: StepperStep[];
  scenarios: StepperScenario[];
}

export interface ScoreCheck {
  id: string;
  name: string;
  target: string;
  weight?: number;
  fixes?: string;
  source?: string;
}

export interface ScoreResult {
  id: string;
  score: number | null;
  observed?: string;
  evidence?: string;
}

export interface ScoreResults {
  generated_at?: string;
  subject?: string;
  results: ScoreResult[];
}

export interface ScorecardData {
  title?: string;
  note?: string;
  checks: ScoreCheck[];
  example?: ScoreResults;
}

export type Widget =
  | { name: "flow"; data: FlowData }
  | { name: "stepper"; data: StepperData }
  | { name: "scorecard"; data: ScorecardData };

export type Validation =
  { ok: true; widget: Widget } | { ok: false; errors: string[] };

/*
 * A field's Go type, as yaml.v3 decodes into it. `go` is the name a
 * mismatch error gives it; "" for the package's own struct types, whose
 * names documentErrors drops.
 */
type Kind =
  | { k: "string" }
  | { k: "int" }
  | { k: "float"; ptr: boolean }
  | { k: "list"; of: Kind; go: string }
  | { k: "strmap" }
  | { k: "struct"; fields: readonly Field[]; ptr: boolean }
  | { k: "result" };

/** A struct field: its key (YAML and JSON alike), type, and omitempty. */
interface Field {
  key: string;
  kind: Kind;
  omitempty: boolean;
}

const STRING: Kind = { k: "string" };

function field(key: string, kind: Kind, omitempty = false): Field {
  return { key, kind, omitempty };
}

function struct(fields: readonly Field[], ptr = false): Kind {
  return { k: "struct", fields, ptr };
}

const FLOW = struct([
  field("title", STRING, true),
  field(
    "stages",
    {
      k: "list",
      go: "",
      of: struct([
        field("name", STRING),
        field("does", STRING),
        field("note", STRING, true),
      ]),
    },
    false,
  ),
  field("gates", { k: "list", go: "[]int", of: { k: "int" } }, true),
  field("label", STRING, true),
  field("legend", STRING, true),
  field("alert", STRING, true),
]);

const STEPPER = struct([
  field("title", STRING, true),
  field("steps", {
    k: "list",
    go: "",
    of: struct([
      field("key", STRING),
      field("name", STRING),
      field("title", STRING, true),
      field("does", STRING),
    ]),
  }),
  field("scenarios", {
    k: "list",
    go: "",
    of: struct([
      field("name", STRING),
      field("fails_at", STRING, true),
      field("fail_label", STRING, true),
      field("notes", { k: "strmap" }, true),
    ]),
  }),
]);

const RESULT_FIELDS: readonly Field[] = [
  field("id", STRING),
  field("score", { k: "float", ptr: true }),
  field("observed", STRING, true),
  field("evidence", STRING, true),
];

const SCORECARD = struct([
  field("title", STRING, true),
  field("note", STRING, true),
  field("checks", {
    k: "list",
    go: "",
    of: struct([
      field("id", STRING),
      field("name", STRING),
      field("target", STRING),
      field("weight", { k: "float", ptr: true }, true),
      field("fixes", STRING, true),
      field("source", STRING, true),
    ]),
  }),
  field(
    "example",
    struct(
      [
        field("generated_at", STRING, true),
        field("subject", STRING, true),
        field("results", { k: "list", go: "", of: { k: "result" } }),
      ],
      true,
    ),
    true,
  ),
]);

const ROOTS: Readonly<Record<WidgetName, Kind>> = {
  flow: FLOW,
  stepper: STEPPER,
  scorecard: SCORECARD,
};

/** The Go type name a mismatch error gives a kind. */
function goName(kind: Kind): string {
  switch (kind.k) {
    case "string":
      return "string";
    case "int":
      return "int";
    case "float":
      return "float64";
    case "list":
      return kind.go;
    case "strmap":
      return "map[string]string";
    default:
      return "";
  }
}

/*
 * A decoded value. `undefined` is Go's nil (an absent slice, map, or
 * pointer); `present` tracks the result's score, which may be null.
 */
type Value =
  string | number | null | undefined | Value[] | { [key: string]: Value };

class Decoder {
  readonly errors: string[] = [];
  private readonly doc: Document;
  private readonly lines: LineCounter;

  constructor(doc: Document, lines: LineCounter) {
    this.doc = doc;
    this.lines = lines;
  }

  private line(node: YamlNode): number {
    return this.lines.linePos(node.range?.[0] ?? 0).line;
  }

  private resolve(node: unknown): YamlNode | null {
    if (isAlias(node)) {
      return (node.resolve(this.doc) as YamlNode | undefined) ?? null;
    }
    return (node as YamlNode | null) ?? null;
  }

  /** yaml.v3's short tag for a node. */
  private tag(node: YamlNode): string {
    if (isMap(node)) {
      return "!!map";
    }
    if (isSeq(node)) {
      return "!!seq";
    }
    if (!isScalar(node)) {
      return "!!str";
    }
    const v = node.value;
    if (v === null) {
      return "!!null";
    }
    if (typeof v === "boolean") {
      return "!!bool";
    }
    if (typeof v === "number") {
      return /^[-+]?(?:0x[0-9a-fA-F]+|0o[0-7]+|[0-9]+)$/.test(this.text(node))
        ? "!!int"
        : "!!float";
    }
    return "!!str";
  }

  /*
   * A scalar's text as yaml.v3 holds it: the parsed string for a string,
   * the source as written for a number or boolean (so `1.50` stays
   * `1.50`), "" for null.
   */
  private text(node: YamlNode): string {
    if (!isScalar(node)) {
      return "";
    }
    if (typeof node.value === "string") {
      return node.value;
    }
    if (node.value === null) {
      return "";
    }
    const source = node.srcToken;
    if (source !== undefined && "source" in source) {
      return source.source;
    }
    return typeof node.value === "number" || typeof node.value === "boolean"
      ? String(node.value)
      : "";
  }

  // Records a mismatch; the value it returns is never read, since a
  // block with any decode error is not checked or drawn.
  private mismatch(node: YamlNode, kind: Kind): Value {
    const tag = this.tag(node);
    const value = isScalar(node) ? ` \`${this.text(node)}\`` : "";
    const into = goName(kind);
    this.errors.push(
      `line ${String(this.line(node))}: cannot unmarshal ${tag}${value}${
        into === "" ? "" : ` into ${into}`
      }`,
    );
    return undefined;
  }

  private isNull(node: YamlNode | null): boolean {
    return node === null || (isScalar(node) && node.value === null);
  }

  decode(raw: unknown, kind: Kind): Value {
    const node = this.resolve(raw);
    if (node === null || this.isNull(node)) {
      return zero(kind);
    }
    switch (kind.k) {
      case "string":
        return isScalar(node) ? this.text(node) : this.mismatch(node, kind);
      case "int":
        return this.number(node, kind, true);
      case "float":
        return this.number(node, kind, false);
      case "list":
        if (!isSeq(node)) {
          return this.mismatch(node, kind);
        }
        return node.items.map((item) => this.decode(item, kind.of));
      case "strmap":
        return this.strmap(node, kind);
      case "struct":
        return this.struct(node, kind.fields, kind);
      case "result":
        return this.result(node);
    }
  }

  private number(node: YamlNode, kind: Kind, int: boolean): Value {
    if (!isScalar(node) || typeof node.value !== "number") {
      return this.mismatch(node, kind);
    }
    if (int && !Number.isInteger(node.value)) {
      return Math.trunc(node.value);
    }
    return node.value;
  }

  private strmap(node: YamlNode, kind: Kind): Value {
    if (!isMap(node)) {
      return this.mismatch(node, kind);
    }
    const out: Record<string, Value> = {};
    for (const pair of node.items) {
      const key = this.resolve(pair.key);
      out[key === null ? "" : this.text(key)] = this.decode(pair.value, STRING);
    }
    return out;
  }

  private struct(node: YamlNode, fields: readonly Field[], kind: Kind): Value {
    if (!isMap(node)) {
      return this.mismatch(node, kind);
    }
    const out = zeroStruct(fields);
    for (const pair of node.items) {
      const key = this.resolve(pair.key);
      const name = key === null ? "" : this.text(key);
      const f = fields.find((candidate) => candidate.key === name);
      if (f === undefined) {
        this.errors.push(
          `line ${String(key === null ? 0 : this.line(key))}: field ${name} not found`,
        );
        continue;
      }
      out[name] = this.decode(pair.value, f.kind);
    }
    return out;
  }

  // A result reads its own mapping in Go, so it checks its keys before
  // decoding any value and stops at an unknown one.
  private result(node: YamlNode): Value {
    if (!isMap(node)) {
      this.errors.push(
        `line ${String(this.line(node))}: a result is a mapping`,
      );
      return undefined;
    }
    let unknown = false;
    let hasScore = false;
    for (const pair of node.items) {
      const key = this.resolve(pair.key);
      const name = key === null ? "" : this.text(key);
      if (name === "score") {
        hasScore = true;
      }
      if (!RESULT_FIELDS.some((f) => f.key === name)) {
        unknown = true;
        this.errors.push(
          `line ${String(key === null ? 0 : this.line(key))}: field ${name} not found`,
        );
      }
    }
    if (unknown) {
      return undefined;
    }
    const out = this.struct(node, RESULT_FIELDS, { k: "result" }) as Record<
      string,
      Value
    >;
    out[HAS_SCORE] = hasScore ? 1 : 0;
    return out;
  }
}

/** A result's private marker that its mapping named score at all. */
const HAS_SCORE = "\u0000hasScore";

function zero(kind: Kind): Value {
  switch (kind.k) {
    case "string":
      return "";
    case "int":
      return 0;
    case "float":
      return kind.ptr ? null : 0;
    case "struct":
      return kind.ptr ? undefined : zeroStruct(kind.fields);
    case "result":
      return { ...zeroStruct(RESULT_FIELDS), [HAS_SCORE]: 0 };
    default:
      return undefined;
  }
}

function zeroStruct(fields: readonly Field[]): Record<string, Value> {
  const out: Record<string, Value> = {};
  for (const f of fields) {
    out[f.key] = zero(f.kind);
  }
  return out;
}

/*
 * The JSON island Go would write: fields in struct order, omitempty
 * dropping "", an empty slice or map, and a nil pointer; a nil slice
 * without omitempty is null, as encoding/json writes it.
 */
function toJSON(value: Value, kind: Kind): unknown {
  switch (kind.k) {
    case "list":
      return Array.isArray(value)
        ? value.map((item) => toJSON(item, kind.of))
        : null;
    case "struct":
      return value === undefined || value === null
        ? null
        : jsonStruct(value as Record<string, Value>, kind.fields);
    case "result":
      return jsonStruct(value as Record<string, Value>, RESULT_FIELDS);
    case "float":
      return value ?? null;
    default:
      return value ?? null;
  }
}

function jsonStruct(
  value: Record<string, Value>,
  fields: readonly Field[],
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const f of fields) {
    const v = value[f.key];
    if (f.omitempty && isEmpty(v)) {
      continue;
    }
    out[f.key] = toJSON(v, f.kind);
  }
  return out;
}

function isEmpty(v: Value): boolean {
  if (v === undefined || v === null || v === "") {
    return true;
  }
  if (Array.isArray(v)) {
    return v.length === 0;
  }
  if (typeof v === "object") {
    return Object.keys(v).length === 0;
  }
  return false;
}

/* The field checks, worded as widgets.go words them. */

/*
 * A widget's data as the checks see it: Go runs them on the decoded
 * struct, where a required list the block left out is nil, which the
 * JSON form holds as null.
 */
type Unchecked<T> = {
  [K in keyof T]: T[K] extends readonly unknown[] ? T[K] | null : T[K];
};

function quote(s: string): string {
  return JSON.stringify(s);
}

function required(
  errs: string[],
  at: string,
  pairs: readonly [string, string][],
): void {
  for (const [key, value] of pairs) {
    if (value.trim() === "") {
      errs.push(`${at}.${key}: required`);
    }
  }
}

function checkFlow(f: Unchecked<FlowData>): string[] {
  const errs: string[] = [];
  const stages = f.stages ?? [];
  const n = stages.length;
  if (n < 2 || n > 8) {
    errs.push(`stages: needs 2 to 8 stages, has ${String(n)}`);
  }
  stages.forEach((s, i) => {
    required(errs, `stages[${String(i)}]`, [
      ["name", s.name],
      ["does", s.does],
    ]);
  });
  for (const g of f.gates ?? []) {
    if (g < 1 || g > n) {
      errs.push(
        `gates: ${String(g)} is not a stage between 1 and ${String(n)}`,
      );
    }
  }
  return errs;
}

const STEP_KEY = /^[a-z0-9-]+$/;

function checkStepper(s: Unchecked<StepperData>): string[] {
  const errs: string[] = [];
  const steps = s.steps ?? [];
  const scenarios = s.scenarios ?? [];
  const n = steps.length;
  if (n < 2 || n > 12) {
    errs.push(`steps: needs 2 to 12 steps, has ${String(n)}`);
  }
  const keys = new Set<string>();
  steps.forEach((st, i) => {
    const at = `steps[${String(i)}]`;
    required(errs, at, [
      ["key", st.key],
      ["name", st.name],
      ["does", st.does],
    ]);
    if (st.key !== "") {
      if (!STEP_KEY.test(st.key)) {
        errs.push(`${at}.key: ${quote(st.key)} is not [a-z0-9-]+`);
      } else if (keys.has(st.key)) {
        errs.push(`${at}.key: ${quote(st.key)} is used twice`);
      }
    }
    keys.add(st.key);
  });
  if (scenarios.length === 0) {
    errs.push("scenarios: needs at least one scenario");
  }
  const names = new Set<string>();
  scenarios.forEach((sc, i) => {
    const at = `scenarios[${String(i)}]`;
    required(errs, at, [["name", sc.name]]);
    if (sc.name !== "" && names.has(sc.name)) {
      errs.push(`${at}.name: ${quote(sc.name)} is used twice`);
    }
    names.add(sc.name);
    if (
      sc.fails_at !== undefined &&
      sc.fails_at !== "" &&
      !keys.has(sc.fails_at)
    ) {
      errs.push(`${at}.fails_at: ${quote(sc.fails_at)} is no step's key`);
    }
    for (const k of Object.keys(sc.notes ?? {}).sort()) {
      if (!keys.has(k)) {
        errs.push(`${at}.notes: ${quote(k)} is no step's key`);
      }
    }
  });
  return errs;
}

function checkScorecard(
  c: Unchecked<ScorecardData>,
  hasScore: readonly boolean[],
): string[] {
  const errs: string[] = [];
  const checks = c.checks ?? [];
  if (checks.length === 0) {
    errs.push("checks: needs at least one check");
  }
  const ids = new Set<string>();
  checks.forEach((ck, i) => {
    const at = `checks[${String(i)}]`;
    required(errs, at, [
      ["id", ck.id],
      ["name", ck.name],
      ["target", ck.target],
    ]);
    if (ck.id !== "" && ids.has(ck.id)) {
      errs.push(`${at}.id: ${quote(ck.id)} is used twice`);
    }
    ids.add(ck.id);
    if (ck.weight !== undefined && ck.weight <= 0) {
      errs.push(`${at}.weight: must be greater than 0`);
    }
  });
  const example = c.example;
  if (example !== undefined) {
    const results = example.results as ScoreResult[] | null;
    if (results === null) {
      errs.push("example.results: required");
    }
    (results ?? []).forEach((res, i) => {
      const at = `example.results[${String(i)}]`;
      required(errs, at, [["id", res.id]]);
      if (res.id !== "" && !ids.has(res.id)) {
        errs.push(`${at}.id: ${quote(res.id)} is no check's id`);
      }
      if (hasScore[i] !== true) {
        errs.push(`${at}.score: required`);
      } else if (res.score !== null && (res.score < 0 || res.score > 1)) {
        errs.push(`${at}.score: must be between 0 and 1, or null`);
      }
    });
  }
  return errs;
}

const LINE = /\bline (\d+)\b/g;
const HAS_LINE = /\bline \d+\b/;

/*
 * documentErrors is widgets.go's: each "line N" of the block moved to
 * the document's line, and an error with no line given the fence's.
 * first is the document line of the block's first YAML line.
 */
export function documentErrors(
  errs: readonly string[],
  first: number,
): string[] {
  return errs.map((e) =>
    HAS_LINE.test(e)
      ? e.replace(
          LINE,
          (_, n: string) => `line ${String(first + Number(n) - 1)}`,
        )
      : `line ${String(first - 1)}: ${e}`,
  );
}

/*
 * validateWidget decodes and checks a block's YAML. On success the
 * widget's data is exactly the JSON island the export writes; on
 * failure the errors carry document lines, given `first`, the document
 * line of the block's first YAML line.
 */
export function validateWidget(
  name: WidgetName,
  source: string,
  first: number,
): Validation {
  const errors = decodeErrors(name, source);
  if (errors.kind === "ok") {
    return { ok: true, widget: errors.widget };
  }
  return { ok: false, errors: documentErrors(errors.errors, first) };
}

type Decoded =
  { kind: "ok"; widget: Widget } | { kind: "errors"; errors: string[] };

function decodeErrors(name: WidgetName, source: string): Decoded {
  const lines = new LineCounter();
  const docs = parseAllDocuments(source, {
    lineCounter: lines,
    keepSourceTokens: true,
    prettyErrors: false,
    uniqueKeys: true,
  });
  const doc = Array.isArray(docs) ? docs[0] : undefined;
  if (doc === undefined) {
    return { kind: "errors", errors: ["the block is empty"] };
  }
  const parseError = doc.errors[0];
  if (parseError !== undefined) {
    // A parser stopped by the end of the block reports the end; yaml.v3
    // reports the last line that has anything on it.
    const end = source.trimEnd().length;
    const line = lines.linePos(
      Math.min(parseError.pos[0], Math.max(end - 1, 0)),
    ).line;
    const message = parseError.message.split("\n")[0] ?? "";
    return { kind: "errors", errors: [`line ${String(line)}: ${message}`] };
  }
  if (doc.contents === null) {
    return { kind: "errors", errors: ["the block is empty"] };
  }

  const root = ROOTS[name];
  const decoder = new Decoder(doc, lines);
  const value = decoder.decode(doc.contents, root);
  if (decoder.errors.length > 0) {
    return { kind: "errors", errors: decoder.errors };
  }

  const data = toJSON(value, root);
  let errs: string[];
  switch (name) {
    case "flow":
      errs = checkFlow(data as Unchecked<FlowData>);
      break;
    case "stepper":
      errs = checkStepper(data as Unchecked<StepperData>);
      break;
    case "scorecard":
      errs = checkScorecard(data as Unchecked<ScorecardData>, scores(value));
      break;
  }
  if (errs.length > 0) {
    return { kind: "errors", errors: errs };
  }
  return { kind: "ok", widget: { name, data } as Widget };
}

/** Whether each example result's mapping named a score at all. */
function scores(value: Value): boolean[] {
  const example = (value as Record<string, Value>).example;
  if (
    example === undefined ||
    example === null ||
    typeof example !== "object"
  ) {
    return [];
  }
  const results = (example as Record<string, Value>).results;
  if (!Array.isArray(results)) {
    return [];
  }
  return results.map(
    (r) =>
      r !== null &&
      typeof r === "object" &&
      (r as Record<string, Value>)[HAS_SCORE] === 1,
  );
}
