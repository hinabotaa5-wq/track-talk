import { describe, expect, it } from "vitest";
import { parseInstagramUrl } from "./instagram";

describe("parseInstagramUrl", () => {
  it("accepts public posts and reels on instagram.com", () => {
    expect(parseInstagramUrl("https://www.instagram.com/p/C1a2B3c4D5e/")).toEqual({ type: "p", shortcode: "C1a2B3c4D5e" });
    expect(parseInstagramUrl("https://instagram.com/reel/C1a2B3c4D5e?igsh=x")).toEqual({ type: "reel", shortcode: "C1a2B3c4D5e" });
    expect(parseInstagramUrl("https://www.instagram.com/reels/C1a2B3c4D5e/")).toEqual({ type: "reel", shortcode: "C1a2B3c4D5e" });
    expect(parseInstagramUrl("https://www.instagram.com/someone/p/C1a2B3c4D5e/")).toEqual({ type: "p", shortcode: "C1a2B3c4D5e" });
  });

  it("rejects other domains and pages", () => {
    for (const raw of [
      "",
      "https://example.com/p/C1a2B3c4D5e/",
      "https://www.instagram.com.evil.com/p/C1a2B3c4D5e/",
      "https://user@www.instagram.com/p/C1a2B3c4D5e/",
      "https://www.instagram.com/someone/",
      "https://www.instagram.com/stories/someone/123/",
      "https://www.instagram.com/p/abc/",
      "javascript:alert(1)",
    ]) {
      expect(parseInstagramUrl(raw)).toBeNull();
    }
  });
});
