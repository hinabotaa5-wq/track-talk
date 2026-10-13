import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { LikeButton, SaveButton } from "./action-button";

describe("LikeButton & SaveButton", () => {
  it("renders like count and inactive state", () => {
    const html = renderToStaticMarkup(
      createElement(LikeButton, {
        liked: false,
        count: 5,
        onClick: () => {},
      })
    );
    expect(html).toContain("いいね");
    expect(html).toContain("5");
  });

  it("renders active like state", () => {
    const html = renderToStaticMarkup(
      createElement(LikeButton, {
        liked: true,
        count: 6,
        onClick: () => {},
      })
    );
    expect(html).toContain("bg-rose-50");
  });

  it("renders save button states", () => {
    const inactive = renderToStaticMarkup(
      createElement(SaveButton, {
        saved: false,
        onClick: () => {},
      })
    );
    expect(inactive).toContain("保存");

    const active = renderToStaticMarkup(
      createElement(SaveButton, {
        saved: true,
        onClick: () => {},
      })
    );
    expect(active).toContain("保存済み");
  });
});
