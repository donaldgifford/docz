import { describe, expect, it } from "vitest";

import { peekReturnTo, stashReturnTo, takeReturnTo } from "@/lib/authReturn";

const RETURN_KEY = "docz:auth:return-to";

describe("auth return-to stash", () => {
  it("round-trips an app path with its query string", () => {
    stashReturnTo("/donaldgifford/docz-api/impl/IMPL-0002?x=1");
    expect(peekReturnTo()).toBe("/donaldgifford/docz-api/impl/IMPL-0002?x=1");
    expect(takeReturnTo()).toBe("/donaldgifford/docz-api/impl/IMPL-0002?x=1");
    // take clears — a restore fires at most once
    expect(peekReturnTo()).toBeNull();
  });

  it("refuses destinations that would loop the round trip", () => {
    stashReturnTo("/");
    stashReturnTo("/login");
    stashReturnTo("/login?next=x");
    expect(sessionStorage.getItem(RETURN_KEY)).toBeNull();
  });

  it("refuses non-app-path shapes (the restore navigates blind)", () => {
    stashReturnTo("https://evil.example/phish");
    stashReturnTo("//evil.example/phish");
    expect(sessionStorage.getItem(RETURN_KEY)).toBeNull();
  });

  it("re-validates on the way out, not just the way in", () => {
    // e.g. another script or an older build wrote something unsafe
    sessionStorage.setItem(RETURN_KEY, "//evil.example/phish");
    expect(takeReturnTo()).toBeNull();
  });
});
