import { describe, expect, it } from "vitest";
import { nextCookiePath, safeNextPath } from "./safe-next";

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

  it("rejects control characters that the URL parser strips", () => {
    expect(safeNextPath("/\t/evil.example")).toBe("/");
    expect(safeNextPath("/\n/evil.example")).toBe("/");
    expect(safeNextPath("/\r/evil.example")).toBe("/");
    expect(nextCookiePath("%2F%09%2Fevil.example")).toBe("/");
  });

  it("does not loop back to the login page", () => {
    expect(safeNextPath("/login")).toBe("/");
    expect(safeNextPath("/login?next=/me")).toBe("/");
  });

  it("reads a same-site return path from a cookie", () => {
    expect(nextCookiePath("%2Fme%2Fdelete")).toBe("/me/delete");
    expect(nextCookiePath("/me/delete")).toBe("/me/delete");
    expect(nextCookiePath("https%3A%2F%2Fevil.example")).toBe("/");
    expect(nextCookiePath(null)).toBe("/");
  });
});
