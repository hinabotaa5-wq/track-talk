import { describe, expect, it } from "vitest";
import { cityPhotoSrc } from "./city-photo";

describe("cityPhotoSrc", () => {
  it("shrinks a large Wikimedia thumbnail and drops tracking params", () => {
    const raw =
      "https://thumb.wikimedia.org/wikipedia/commons/thumb/6/61/Beijing_skyline_at_night.JPG/1280px-Beijing_skyline_at_night.JPG?utm_source=commons&utm_campaign=imageinfo";
    expect(cityPhotoSrc(raw)).toBe(
      "https://thumb.wikimedia.org/wikipedia/commons/thumb/6/61/Beijing_skyline_at_night.JPG/960px-Beijing_skyline_at_night.JPG",
    );
  });

  it("keeps a thumbnail that is already small enough", () => {
    const raw = "https://upload.wikimedia.org/wikipedia/commons/thumb/6/61/Beijing_skyline_at_night.JPG/500px-Beijing_skyline_at_night.JPG?utm_source=commons";
    expect(cityPhotoSrc(raw)).toBe(
      "https://upload.wikimedia.org/wikipedia/commons/thumb/6/61/Beijing_skyline_at_night.JPG/500px-Beijing_skyline_at_night.JPG",
    );
  });

  it("turns an original file into a 960px thumbnail", () => {
    const raw = "https://upload.wikimedia.org/wikipedia/commons/6/61/Beijing_skyline_at_night.JPG";
    expect(cityPhotoSrc(raw)).toBe(
      "https://upload.wikimedia.org/wikipedia/commons/thumb/6/61/Beijing_skyline_at_night.JPG/960px-Beijing_skyline_at_night.JPG",
    );
  });

  it("leaves other hosts alone", () => {
    const raw = "https://example.test/city.jpg";
    expect(cityPhotoSrc(raw)).toBe(raw);
    expect(cityPhotoSrc("")).toBe("");
  });
});
