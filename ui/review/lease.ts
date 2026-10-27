/*
 * The session lease (DESIGN-0024 section 6, Data Model): lease.json holds
 * the pid of the one server reviewing a session and when its claim runs
 * out. A second server refuses while the lease is live and takes over one
 * that expired or whose process is gone. The server renews the lease
 * while the page's event stream is connected, so a closed tab lets it
 * lapse after the grace period and the next Stop can open the session.
 */
import { randomBytes } from "node:crypto";
import { rmSync } from "node:fs";
import { join } from "node:path";

import { readNoFollow, type Session, writeAtomic } from "./state";

export interface LeaseFile {
  pid: number;
  /** Distinguishes two holders that happen to share a pid. */
  owner: string;
  expires_at: string;
}

/** Another live server holds the session. */
export class LeaseHeldError extends Error {
  constructor(readonly holder: LeaseFile) {
    super(
      `another docz-review (pid ${String(holder.pid)}) holds this session until ${holder.expires_at}`,
    );
    this.name = "LeaseHeldError";
  }
}

/** Whether a process exists; EPERM means it does, under another user. */
export function processAlive(pid: number): boolean {
  try {
    process.kill(pid, 0);
    return true;
  } catch (err) {
    return (err as NodeJS.ErrnoException).code === "EPERM";
  }
}

export interface LeaseOptions {
  ttlMs: number;
  pid?: number;
  now?: () => Date;
  alive?: (pid: number) => boolean;
}

export class Lease {
  private constructor(
    private readonly session: Session,
    readonly owner: string,
    private readonly pid: number,
    private readonly now: () => Date,
    private expires: number,
  ) {}

  private static path(session: Session): string {
    return join(session.dir, "lease.json");
  }

  /** The current lease file, or null when there is none. */
  static read(session: Session): LeaseFile | null {
    const raw = readNoFollow(Lease.path(session));
    return raw === null
      ? null
      : (JSON.parse(raw.toString("utf8")) as LeaseFile);
  }

  /**
   * Takes the session's lease, or throws LeaseHeldError while another
   * live server holds it.
   */
  static acquire(session: Session, opts: LeaseOptions): Lease {
    const now = opts.now ?? (() => new Date());
    const pid = opts.pid ?? process.pid;
    const alive = opts.alive ?? processAlive;
    return session.locked(() => {
      const held = Lease.read(session);
      if (
        held !== null &&
        Date.parse(held.expires_at) > now().getTime() &&
        alive(held.pid)
      ) {
        throw new LeaseHeldError(held);
      }
      const lease = new Lease(
        session,
        `l_${randomBytes(8).toString("hex")}`,
        pid,
        now,
        now().getTime() + opts.ttlMs,
      );
      lease.write();
      return lease;
    });
  }

  get expiresAt(): Date {
    return new Date(this.expires);
  }

  /** Whether the claim has run out by the clock. */
  expired(): boolean {
    return this.now().getTime() >= this.expires;
  }

  /**
   * Extends the claim by ttlMs from now. False when the lease was taken
   * over meanwhile, which the server treats as losing the session.
   */
  renew(ttlMs: number): boolean {
    return this.session.locked(() => {
      if (Lease.read(this.session)?.owner !== this.owner) {
        return false;
      }
      this.expires = this.now().getTime() + ttlMs;
      this.write();
      return true;
    });
  }

  /** Gives the session up; a lease another server took is left alone. */
  release(): void {
    this.session.locked(() => {
      if (Lease.read(this.session)?.owner === this.owner) {
        rmSync(Lease.path(this.session), { force: true });
      }
    });
  }

  private write(): void {
    const file: LeaseFile = {
      pid: this.pid,
      owner: this.owner,
      expires_at: new Date(this.expires).toISOString(),
    };
    writeAtomic(Lease.path(this.session), `${JSON.stringify(file)}\n`);
  }
}
