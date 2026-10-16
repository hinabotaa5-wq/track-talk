import { describe, expect, it } from "vitest";
import { canShrink, fitWithin, shrinkImage } from "./shrink-image";

describe("fitWithin", () => {
  it("小さい写真はそのまま", () => {
    expect(fitWithin(1600, 1200)).toEqual({ width: 1600, height: 1200 });
  });

  it("横長は横を2048pxにそろえる", () => {
    expect(fitWithin(4032, 3024)).toEqual({ width: 2048, height: 1536 });
  });

  it("縦長は縦を2048pxにそろえる", () => {
    expect(fitWithin(3024, 4032)).toEqual({ width: 1536, height: 2048 });
  });
});

describe("canShrink", () => {
  it("GIFは縮めない", () => {
    expect(canShrink(new File(["x"], "a.gif", { type: "image/gif" }))).toBe(false);
    expect(canShrink(new File(["x"], "a.GIF", { type: "" }))).toBe(false);
  });

  it("JPEGやHEICは縮める対象", () => {
    expect(canShrink(new File(["x"], "a.jpg", { type: "image/jpeg" }))).toBe(true);
    expect(canShrink(new File(["x"], "a.heic", { type: "image/heic" }))).toBe(true);
  });
});

describe("shrinkImage", () => {
  it("ブラウザの外では元のファイルを返す", async () => {
    const file = new File(["x"], "a.jpg", { type: "image/jpeg" });
    await expect(shrinkImage(file)).resolves.toBe(file);
  });
});
