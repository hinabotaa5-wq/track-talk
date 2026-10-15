import { describe, expect, it } from "vitest";
import { base64UrlToBytes, formatYen } from "./push";

describe("base64UrlToBytes", () => {
  it("decodes unpadded base64url", () => {
    expect(Array.from(base64UrlToBytes("-_8"))).toEqual([0xfb, 0xff]);
    expect(Array.from(base64UrlToBytes("AQID"))).toEqual([1, 2, 3]);
  });
});

describe("formatYen", () => {
  it("formats the monthly limit", () => {
    expect(formatYen(3000)).toBe("月3,000円まで");
    expect(formatYen(null)).toBe("上限なし");
  });
});
