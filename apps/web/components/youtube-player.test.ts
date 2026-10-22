import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { YouTubePlayer } from "./youtube-player";

describe("YouTubePlayer", () => {
  it("shows YouTube's own player without a second play button on top", () => {
    const html = renderToStaticMarkup(createElement(YouTubePlayer, { videoId: "dQw4w9WgXcQ", title: "テスト動画" }));
    expect(html).toContain('src="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?playsinline=1&amp;rel=0&amp;cc_load_policy=0"');
    expect(html).toContain('title="テスト動画"');
    expect(html).not.toContain("<button");
  });

  it("Shorts は縦長の枠で描く", () => {
    const html = renderToStaticMarkup(createElement(YouTubePlayer, { videoId: "dQw4w9WgXcQ", title: "テスト", shorts: true }));
    expect(html).toContain("aspect-[9/16]");
    expect(html).not.toContain("aspect-video");
  });

  it("ふつうの動画は横のまま", () => {
    const html = renderToStaticMarkup(createElement(YouTubePlayer, { videoId: "dQw4w9WgXcQ", title: "テスト" }));
    expect(html).toContain("aspect-video");
    expect(html).not.toContain("aspect-[9/16]");
  });
});
