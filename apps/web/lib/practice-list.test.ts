import { describe, expect, it } from "vitest";
import { practiceCountLabel, practiceFilterFrom, practiceListPath, practicePagePath } from "./practice-list";

describe("practice list filters", () => {
  it("defaults to all prefectures and events and ignores unknown values", () => {
    expect(practiceFilterFrom(null, null)).toEqual({ prefecture: "", event: "" });
    expect(practiceFilterFrom("00", "sprint")).toEqual({ prefecture: "", event: "" });
    expect(practiceFilterFrom("13", "100m")).toEqual({ prefecture: "13", event: "100m" });
  });

  it("builds the API and page paths, leaving out filters that are all", () => {
    expect(practiceListPath({ prefecture: "", event: "" })).toBe("/v1/practice");
    expect(practiceListPath({ prefecture: "13", event: "400m" }, "abc")).toBe("/v1/practice?prefecture=13&event=400m&cursor=abc");
    expect(practicePagePath({ prefecture: "", event: "100m" })).toBe("/practice?event=100m");
  });

  it("labels the count with the chosen area and event", () => {
    expect(practiceCountLabel({ prefecture: "", event: "" }, 5)).toBe("全国 これからの募集 5件");
    expect(practiceCountLabel({ prefecture: "13", event: "100m" }, 3)).toBe("東京都の100m これからの募集 3件");
  });
});
