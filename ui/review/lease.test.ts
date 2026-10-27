import { describe, expect, test } from "bun:test";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { Lease, LeaseHeldError, processAlive } from "./lease";
import { Session } from "./state";

const session = (now?: () => Date) =>
  new Session({
    base: mkdtempSync(join(tmpdir(), "docz-review-lease-")),
    root: "/repo",
    session: "s",
    now,
  });

describe("Lease", () => {
  test("refuses a second server while the lease is live", () => {
    const s = session();
    const first = Lease.acquire(s, { ttlMs: 60_000 });
    expect(() => Lease.acquire(s, { ttlMs: 60_000 })).toThrow(LeaseHeldError);
    expect(Lease.read(s)).toMatchObject({
      pid: process.pid,
      owner: first.owner,
    });
  });

  test("takes over an expired lease, and the old holder cannot renew it", () => {
    let now = new Date("2026-10-25T12:00:00Z");
    const clock = () => now;
    const s = session(clock);
    const first = Lease.acquire(s, { ttlMs: 1000, now: clock });
    now = new Date(now.getTime() + 1001);
    expect(first.expired()).toBe(true);
    const second = Lease.acquire(s, { ttlMs: 1000, now: clock });
    expect(first.renew(1000)).toBe(false);
    expect(second.renew(5000)).toBe(true);
    expect(second.expiresAt.getTime()).toBe(now.getTime() + 5000);
  });

  test("takes over a live lease whose process is gone", () => {
    const s = session();
    Lease.acquire(s, { ttlMs: 60_000, pid: 1 << 22 });
    const lease = Lease.acquire(s, { ttlMs: 60_000, alive: () => false });
    expect(Lease.read(s)?.owner).toBe(lease.owner);
  });

  test("release removes only the releaser's own lease", () => {
    let now = new Date("2026-10-25T12:00:00Z");
    const clock = () => now;
    const s = session(clock);
    const first = Lease.acquire(s, { ttlMs: 10, now: clock });
    now = new Date(now.getTime() + 20);
    const second = Lease.acquire(s, { ttlMs: 10_000, now: clock });
    first.release();
    expect(Lease.read(s)?.owner).toBe(second.owner);
    second.release();
    expect(Lease.read(s)).toBeNull();
  });
});

describe("processAlive", () => {
  test("knows this process and not an unused pid", () => {
    expect(processAlive(process.pid)).toBe(true);
    expect(processAlive(1 << 22)).toBe(false);
  });
});
