import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { InstagramEmbed } from "./instagram-embed";

describe("InstagramEmbed", () => {
  it("embeds the official instagram.com frame with a link to open it", () => {
    const html = renderToStaticMarkup(createElement(InstagramEmbed, { type: "reel", shortcode: "C1a2B3c4D5e", title: "リレー" }));
    expect(html).toContain('src="https://www.instagram.com/reel/C1a2B3c4D5e/embed/"');
    expect(html).toContain('href="https://www.instagram.com/reel/C1a2B3c4D5e/"');
    expect(html).toContain("Instagramで見る");
  });

  it("renders nothing for a broken shortcode", () => {
    const html = renderToStaticMarkup(createElement(InstagramEmbed, { type: "p", shortcode: "x\"/><script>", title: "x" }));
    expect(html).toBe("");
  });
});
