import { describe, expect, it } from "vitest";
import { base64UrlToBytes, monthlyLimitLabel, subscriptionMatchesKey } from "./push";

describe("base64UrlToBytes", () => {
  it("decodes unpadded base64url", () => {
    expect(Array.from(base64UrlToBytes("-_8"))).toEqual([0xfb, 0xff]);
    expect(Array.from(base64UrlToBytes("AQID"))).toEqual([1, 2, 3]);
  });
});

describe("monthlyLimitLabel", () => {
  it("formats the monthly limit", () => {
    expect(monthlyLimitLabel(3000)).toBe("月3,000円まで");
    expect(monthlyLimitLabel(null)).toBe("上限なし");
  });
});

describe("subscriptionMatchesKey", () => {
  const withKey = (bytes: number[] | null) =>
    ({ options: { applicationServerKey: bytes ? new Uint8Array(bytes).buffer : null, userVisibleOnly: true } }) as Pick<PushSubscription, "options">;

  it("keeps a subscription made with the current key", () => {
    expect(subscriptionMatchesKey(withKey([1, 2, 3]), "AQID")).toBe(true);
  });

  it("detects a subscription made with a previous key", () => {
    expect(subscriptionMatchesKey(withKey([1, 2, 4]), "AQID")).toBe(false);
    expect(subscriptionMatchesKey(withKey([1, 2]), "AQID")).toBe(false);
  });

  it("does not recreate when the browser does not report the key", () => {
    expect(subscriptionMatchesKey(withKey(null), "AQID")).toBe(true);
  });
});
