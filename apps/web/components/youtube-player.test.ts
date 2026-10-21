import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { YouTubePlayer } from "./youtube-player";

describe("YouTubePlayer", () => {
  it("shows YouTube's own player without a second play button on top", () => {
    const html = renderToStaticMarkup(createElement(YouTubePlayer, { videoId: "dQw4w9WgXcQ", title: "テスト動画" }));
    expect(html).toContain('src="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?playsinline=1&amp;rel=0"');
    expect(html).toContain('title="テスト動画"');
    expect(html).not.toContain("<button");
  });
});
