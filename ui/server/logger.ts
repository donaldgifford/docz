/*
 * Structured logging (DESIGN-0006 Component 3).
 *
 * Hand-written rather than pino: it is small, it keeps the logging path
 * dependency-free, and the redaction rules in redact.ts want to be
 * applied at the call site by construction rather than bolted onto a
 * general-purpose serialiser.
 *
 * One object per line to stdout. JSON by default, because container
 * logs are almost always machine-read; `text` stays available for local
 * runs.
 *
 * Nothing here redacts anything — callers pass values that have already
 * been through redact.ts. Keeping that boundary sharp is deliberate: a
 * logger that redacts is a logger someone will eventually trust with a
 * raw URL.
 */

/** Severities, ordered. */
export const LOG_LEVELS = ["debug", "info", "warn", "error"] as const;
export type LogLevel = (typeof LOG_LEVELS)[number];

/** Output shapes. */
export const LOG_FORMATS = ["json", "text"] as const;
export type LogFormat = (typeof LOG_FORMATS)[number];

/** Field values a log event may carry. `undefined` fields are omitted. */
export type LogValue = string | number | boolean | undefined;
export type LogFields = Readonly<Record<string, LogValue>>;

export interface Logger {
  debug(event: string, fields?: LogFields): void;
  info(event: string, fields?: LogFields): void;
  warn(event: string, fields?: LogFields): void;
  error(event: string, fields?: LogFields): void;
  /** The configured level, so callers can skip expensive field building. */
  readonly level: LogLevel;
}

export interface LoggerOptions {
  level: LogLevel;
  format: LogFormat;
  /**
   * Sink for finished lines. Defaults to stdout. Tests pass a capture
   * function so they assert on the SERIALISED line rather than on the
   * fields handed in — the difference that makes a redaction test
   * meaningful.
   */
  write?: (line: string) => void;
  /** Injectable clock, so tests are not time-dependent. */
  now?: () => Date;
}

const SEVERITY: Record<LogLevel, number> = {
  debug: 10,
  info: 20,
  warn: 30,
  error: 40,
};

/**
 * Render one field for text mode. Strings are JSON-quoted so a value
 * containing a space can never look like two fields.
 */
function textValue(value: Exclude<LogValue, undefined>): string {
  return typeof value === "string" ? JSON.stringify(value) : String(value);
}

function formatText(
  timestamp: string,
  level: LogLevel,
  event: string,
  fields: LogFields,
): string {
  const parts = [timestamp, level.toUpperCase(), event];
  for (const [key, value] of Object.entries(fields)) {
    if (value !== undefined) {
      parts.push(`${key}=${textValue(value)}`);
    }
  }
  return parts.join(" ");
}

function formatJson(
  timestamp: string,
  level: LogLevel,
  event: string,
  fields: LogFields,
): string {
  const record: Record<string, LogValue> = { ts: timestamp, level, event };
  for (const [key, value] of Object.entries(fields)) {
    if (value !== undefined) {
      record[key] = value;
    }
  }
  return JSON.stringify(record);
}

export function createLogger(options: LoggerOptions): Logger {
  const { level, format } = options;
  const write =
    options.write ??
    ((line: string) => {
      console.log(line);
    });
  const now = options.now ?? (() => new Date());
  const threshold = SEVERITY[level];

  function emit(at: LogLevel, event: string, fields: LogFields = {}): void {
    if (SEVERITY[at] < threshold) {
      return;
    }
    const timestamp = now().toISOString();
    write(
      format === "json"
        ? formatJson(timestamp, at, event, fields)
        : formatText(timestamp, at, event, fields),
    );
  }

  return {
    level,
    debug: (event, fields) => {
      emit("debug", event, fields);
    },
    info: (event, fields) => {
      emit("info", event, fields);
    },
    warn: (event, fields) => {
      emit("warn", event, fields);
    },
    error: (event, fields) => {
      emit("error", event, fields);
    },
  };
}
