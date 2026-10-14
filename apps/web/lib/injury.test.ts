import { describe, expect, it } from "vitest";
import { injuryFeedPath, isInjuryBodyPart, isInjuryMood } from "./injury";

describe("injury", () => {
  it("builds the feed path with only valid filters", () => {
    expect(injuryFeedPath("", "", 1)).toBe("/v1/posts?kind=injury&sort=new&page=1&per_page=20");
    expect(injuryFeedPath("vent", "knee", 2)).toBe(
      "/v1/posts?kind=injury&sort=new&page=2&per_page=20&mood=vent&part=knee",
    );
    expect(injuryFeedPath("angry", "head", 1)).toBe("/v1/posts?kind=injury&sort=new&page=1&per_page=20");
  });

  it("validates mood and body part codes", () => {
    expect(isInjuryMood("rehab")).toBe(true);
    expect(isInjuryMood("other")).toBe(false);
    expect(isInjuryBodyPart("lower-back")).toBe(true);
    expect(isInjuryBodyPart(null)).toBe(false);
  });
});
