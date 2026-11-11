import { describe, expect, it } from "vitest";
import { flagEnabled, flagEnabledDefaultOn } from "./features";

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

describe("flagEnabledDefaultOn", () => {
  it("shows when unset or empty", () => {
    expect(flagEnabledDefaultOn(undefined)).toBe(true);
    expect(flagEnabledDefaultOn("")).toBe(true);
    expect(flagEnabledDefaultOn("   ")).toBe(true);
  });

  it("shows for other values", () => {
    expect(flagEnabledDefaultOn("1")).toBe(true);
    expect(flagEnabledDefaultOn(" TRUE ")).toBe(true);
    expect(flagEnabledDefaultOn("yes")).toBe(true);
  });

  it("hides only on explicit off", () => {
    expect(flagEnabledDefaultOn("0")).toBe(false);
    expect(flagEnabledDefaultOn("false")).toBe(false);
    expect(flagEnabledDefaultOn(" FALSE ")).toBe(false);
    expect(flagEnabledDefaultOn("no")).toBe(false);
    expect(flagEnabledDefaultOn("NO")).toBe(false);
    expect(flagEnabledDefaultOn("off")).toBe(false);
    expect(flagEnabledDefaultOn(" OFF ")).toBe(false);
  });
});
