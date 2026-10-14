import { describe, expect, it } from "vitest";
import { homeHref, parseHomeSort, parseProductKind } from "./home-sort";

describe("review tab", () => {
  it("parses the review sort", () => {
    expect(parseHomeSort("review")).toBe("review");
    expect(parseHomeSort("unknown")).toBe("popular");
  });

  it("defaults the product kind to shoe", () => {
    expect(parseProductKind(null)).toBe("shoe");
    expect(parseProductKind("spike")).toBe("spike");
  });

  it("builds review links with the kind only for spikes", () => {
    expect(homeHref("review", "")).toBe("/?sort=review");
    expect(homeHref("review", "", undefined, "spike")).toBe("/?sort=review&type=spike");
    expect(homeHref("new", "", undefined, "spike")).toBe("/?sort=new");
  });
});
