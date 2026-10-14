import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { YouTubePlayer } from "./youtube-player";

vi.mock("next/image", () => ({
  default: ({ src, alt }: { src: string; alt: string }) => createElement("img", { src, alt }),
}));

describe("YouTubePlayer", () => {
  it("renders poster thumbnail initially without loading iframe", () => {
    const html = renderToStaticMarkup(
      createElement(YouTubePlayer, {
        videoId: "dQw4w9WgXcQ",
        title: "テスト動画",
      })
    );
    expect(html).toContain("https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg");
    expect(html).not.toContain("<iframe");
    expect(html).toContain("テスト動画");
  });

  it("uses custom thumbnail if provided", () => {
    const html = renderToStaticMarkup(
      createElement(YouTubePlayer, {
        videoId: "dQw4w9WgXcQ",
        title: "テスト動画",
        thumbnailUrl: "https://example.com/custom.jpg",
      })
    );
    expect(html).toContain("https://example.com/custom.jpg");
  });
});
