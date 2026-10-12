import { describe, expect, it } from "vitest";
import { safeNextPath } from "./safe-next";

describe("safeNextPath", () => {
  it("keeps same-site paths", () => {
    expect(safeNextPath("/me")).toBe("/me");
    expect(safeNextPath("/competitions?prefecture=13&month=4")).toBe("/competitions?prefecture=13&month=4");
  });

  it("rejects external or malformed targets", () => {
    expect(safeNextPath("https://evil.example")).toBe("/");
    expect(safeNextPath("//evil.example")).toBe("/");
    expect(safeNextPath("/\\evil.example")).toBe("/");
    expect(safeNextPath("javascript:alert(1)")).toBe("/");
    expect(safeNextPath("")).toBe("/");
    expect(safeNextPath(null)).toBe("/");
  });

  it("does not loop back to the login page", () => {
    expect(safeNextPath("/login")).toBe("/");
    expect(safeNextPath("/login?next=/me")).toBe("/");
  });
});
