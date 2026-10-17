import { describe, expect, it } from "vitest";
import {
  activeReviewFilterCount,
  parseReviewFilters,
  reviewAttrsPayload,
  reviewFiltersApiQuery,
  reviewFiltersQuery,
  withReviewFilters,
} from "./review-filters";

describe("review filters", () => {
  it("reads only known values from the URL", () => {
    const filters = parseReviewFilters(new URLSearchParams("event=100m&level=pro&size=25&width=wide&usage=race&q=x"));
    expect(filters).toEqual({ event: "100m", level: undefined, size: 25, width: "wide", usage: "race" });
    expect(activeReviewFilterCount(filters)).toBe(4);
    expect(parseReviewFilters(new URLSearchParams("size=31")).size).toBeUndefined();
    expect(activeReviewFilterCount(parseReviewFilters(new URLSearchParams("")))).toBe(0);
  });

  it("turns a size band into the API's range", () => {
    const api = reviewFiltersApiQuery({ size: 25, width: "wide", level: "high-school" });
    expect(api.get("foot_size_min")).toBe("25.0");
    expect(api.get("foot_size_max")).toBe("25.5");
    expect(api.get("foot_width")).toBe("wide");
    expect(api.get("level")).toBe("high-school");
    expect(api.has("size")).toBe(false);
  });

  it("keeps the filters on links", () => {
    expect(withReviewFilters("/?sort=review", { usage: "race" })).toBe("/?sort=review&usage=race");
    expect(withReviewFilters("/reviews/abc", { event: "100m", size: 26 })).toBe("/reviews/abc?event=100m&size=26");
    expect(withReviewFilters("/reviews/abc", {})).toBe("/reviews/abc");
    // URL に載せたものを読み直すと同じになる。
    const filters = { event: "long-jump", level: "adult", size: 27, width: "narrow", usage: "both" };
    expect(parseReviewFilters(reviewFiltersQuery(filters))).toEqual(filters);
  });

  it("sends only the attributes that were chosen", () => {
    expect(reviewAttrsPayload({ event: "100m", level: "", footSize: "25.5", width: "", usage: "race" })).toEqual({
      event: "100m",
      foot_size: 25.5,
      usage: "race",
    });
    expect(reviewAttrsPayload({ event: "", level: "", footSize: "", width: "", usage: "" })).toEqual({});
  });
});
