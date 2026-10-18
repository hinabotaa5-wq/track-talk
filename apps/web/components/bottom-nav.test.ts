import { describe, expect, it } from "vitest";
import { hidesBottomNav, showsBackButton } from "./bottom-nav";

describe("login page header and bottom nav", () => {
  it("shows the back button only when sent to login from a page that needs it", () => {
    expect(showsBackButton("/login", "?next=%2Fupload")).toBe(true);
    expect(showsBackButton("/login", "")).toBe(false);
    expect(showsBackButton("/login")).toBe(false);
  });

  it("shows the bottom nav on the login page", () => {
    expect(hidesBottomNav("/login")).toBe(false);
    expect(hidesBottomNav("/register")).toBe(true);
  });
});
