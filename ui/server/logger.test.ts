import { describe, expect, test } from "bun:test";

import {
  createLogger,
  LOG_LEVELS,
  type LogFormat,
  type LogLevel,
} from "./logger";

/** Collect serialised lines so assertions see what would hit stdout. */
function capture(level: LogLevel, format: LogFormat = "json") {
  const lines: string[] = [];
  const log = createLogger({
    level,
    format,
    write: (line) => lines.push(line),
    now: () => new Date("2026-09-19T12:00:00.000Z"),
  });
  return { log, lines };
}

describe("level filtering", () => {
  test("info (the default) drops debug but keeps warn and error", () => {
    const { log, lines } = capture("info");
    log.debug("http.request");
    log.info("server.start");
    log.warn("readyz.fail");
    log.error("proxy.error");
    expect(lines).toHaveLength(3);
    expect(lines.join("\n")).not.toContain("http.request");
  });

  test("debug keeps everything", () => {
    const { log, lines } = capture("debug");
    for (const level of LOG_LEVELS) {
      log[level](`event.${level}`);
    }
    expect(lines).toHaveLength(LOG_LEVELS.length);
  });

  test("error drops all three lower levels", () => {
    const { log, lines } = capture("error");
    log.debug("a");
    log.info("b");
    log.warn("c");
    log.error("d");
    expect(lines).toHaveLength(1);
    expect(lines[0]).toContain("d");
  });

  test("the configured level is readable, so callers can skip work", () => {
    expect(capture("warn").log.level).toBe("warn");
  });
});

describe("json format", () => {
  test("emits one parseable object per line with ts, level, event", () => {
    const { log, lines } = capture("info");
    log.info("server.start", { port: 8080, dist: "dist" });
    expect(lines).toHaveLength(1);
    const record = JSON.parse(lines[0] ?? "") as Record<string, unknown>;
    expect(record).toEqual({
      ts: "2026-09-19T12:00:00.000Z",
      level: "info",
      event: "server.start",
      port: 8080,
      dist: "dist",
    });
  });

  test("undefined fields are omitted, not rendered as null", () => {
    const { log, lines } = capture("info");
    log.info("proxy.request", {
      target_host: "api:8080",
      location_host: undefined,
    });
    const record = JSON.parse(lines[0] ?? "") as Record<string, unknown>;
    expect(record).not.toHaveProperty("location_host");
    expect(record.target_host).toBe("api:8080");
  });

  test("a value containing a newline cannot split the line", () => {
    const { log, lines } = capture("info");
    log.info("http.request", { path: `/a${String.fromCharCode(10)}b` });
    expect(lines).toHaveLength(1);
    expect(lines[0]).not.toContain(String.fromCharCode(10));
  });
});

describe("text format", () => {
  test("renders timestamp, level, event, then fields", () => {
    const { log, lines } = capture("info", "text");
    log.warn("readyz.fail", { check: "dist" });
    expect(lines[0]).toBe(
      '2026-09-19T12:00:00.000Z WARN readyz.fail check="dist"',
    );
  });

  test("string values are quoted, so a space cannot look like two fields", () => {
    const { log, lines } = capture("info", "text");
    log.error("proxy.error", { err_message: "connection refused" });
    expect(lines[0]).toContain('err_message="connection refused"');
  });

  test("numbers and booleans render unquoted", () => {
    const { log, lines } = capture("info", "text");
    log.info("http.request", { status: 502, has_cookie: true });
    expect(lines[0]).toContain("status=502");
    expect(lines[0]).toContain("has_cookie=true");
  });
});
