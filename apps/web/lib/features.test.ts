import { describe, expect, it } from "vitest";
import { flagEnabled } from "./features";

describe("flagEnabled", () => {
  it("is off unless explicitly turned on", () => {
    expect(flagEnabled(undefined)).toBe(false);
    expect(flagEnabled("")).toBe(false);
    expect(flagEnabled("0")).toBe(false);
    expect(flagEnabled("false")).toBe(false);
  });

  it("accepts the usual truthy spellings", () => {
    expect(flagEnabled("1")).toBe(true);
    expect(flagEnabled("true")).toBe(true);
    expect(flagEnabled(" TRUE ")).toBe(true);
  });
});
