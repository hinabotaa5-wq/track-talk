import { describe, expect, it } from "vitest";
import { detectVideoUrl } from "./video-url";

describe("detectVideoUrl", () => {
  it("YouTube の URL を見分ける", () => {
    expect(detectVideoUrl("https://www.youtube.com/watch?v=abc123")).toBe("youtube");
    expect(detectVideoUrl("https://youtu.be/abc123")).toBe("youtube");
    expect(detectVideoUrl(" https://youtube.com/shorts/abc123 ")).toBe("youtube");
  });

  it("Instagram の投稿とリールを見分ける", () => {
    expect(detectVideoUrl("https://www.instagram.com/p/Cabc12345/")).toBe("instagram");
    expect(detectVideoUrl("https://www.instagram.com/reel/Cabc12345/?igsh=x")).toBe("instagram");
  });

  it("どちらでもない URL は null", () => {
    expect(detectVideoUrl("")).toBeNull();
    expect(detectVideoUrl("https://example.com/watch?v=abc")).toBeNull();
    expect(detectVideoUrl("https://www.instagram.com/someone/")).toBeNull();
  });
});
