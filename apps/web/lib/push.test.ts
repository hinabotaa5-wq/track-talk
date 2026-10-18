import { describe, expect, it } from "vitest";
import { base64UrlToBytes, monthlyLimitLabel } from "./push";

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
