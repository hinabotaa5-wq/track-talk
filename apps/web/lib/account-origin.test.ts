/** @vitest-environment jsdom */
import { beforeEach, expect, it, vi } from "vitest";
import { readAccountOrigin, rememberAccountOrigin } from "./account-origin";

beforeEach(() => {
  sessionStorage.clear();
});

it("未保存のときはコーチを返す", () => {
  expect(readAccountOrigin()).toBe("/coaching");
});

it.each(["//example.com", "/me", "/me/profile", "/login", "/login?next=/memo", "memo", ""])("不正な保存値はコーチに寄せる: %s", (saved) => {
  sessionStorage.setItem("account-origin", saved);
  expect(readAccountOrigin()).toBe("/coaching");
});

it.each(["/", "/memo", "/injury?tab=data", "/competitions", "/coaching"])("有効な保存値はそのまま返す: %s", (saved) => {
  rememberAccountOrigin(saved);
  expect(readAccountOrigin()).toBe(saved);
});

it("保存が制限されていてもコーチを返して起動を妨げない", () => {
  vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("blocked"); });
  expect(readAccountOrigin()).toBe("/coaching");
  vi.restoreAllMocks();
});
