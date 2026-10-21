import { describe, expect, it } from "vitest";
import { currentRanking } from "./product";

describe("currentRanking", () => {
  const shoe = { kind: "shoe" as const, event: "100m", products: [] };

  it("開いている種類と種目のランキングを出す", () => {
    expect(currentRanking(shoe, "shoe", "100m")).toBe(shoe);
  });

  it("種類を切り替えた直後は、前の種類のランキングを出さない", () => {
    expect(currentRanking(shoe, "spike", "100m")).toBeNull();
  });

  it("種目が違えば出さない", () => {
    expect(currentRanking(shoe, "shoe", "400m")).toBeNull();
    expect(currentRanking(null, "shoe", "100m")).toBeNull();
  });
});
