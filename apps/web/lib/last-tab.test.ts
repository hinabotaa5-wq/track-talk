import { runInNewContext } from "node:vm";
import { expect, it, vi } from "vitest";
import { lastTabStartupScript, resolveColdStartTarget } from "./last-tab";

function start(saved: string | null, url = "/", blocked = false, requestAnimationFrame?: (callback: () => void) => void) {
  const parsed = new URL(url, "https://track-talk.netlify.app");
  const replace = vi.fn();
  const window = { __tracktalkLastTabRedirect: false };
  const getItem = vi.fn(() => {
    if (blocked) throw new Error("blocked");
    return saved;
  });
  runInNewContext(lastTabStartupScript, {
    location: { pathname: parsed.pathname, search: parsed.search, hash: parsed.hash, replace },
    localStorage: { getItem }, window, URLSearchParams, requestAnimationFrame,
  });
  return { replace, window, getItem };
}

it.each([
  "/memo",
  "/coaching",
  "/injury?tab=data",
  "/competitions?view=international",
  "/",
  "/?sort=review&type=spike&q=secret",
  "https://example.com",
  "//example.com",
  "/login?next=/memo",
  "/posts/123",
  "constructor",
  "/injury#bad",
  null,
])("保存値にかかわらずコーチへ移動する: %s", (saved) => {
  const result = start(saved);
  expect(result.replace).toHaveBeenCalledExactlyOnceWith("/coaching");
  expect(result.window.__tracktalkLastTabRedirect).toBe(true);
  expect(result.getItem).not.toHaveBeenCalled();
});

it("起動ロゴを一度描画してから移動し、その間も二重遷移を防ぐ", () => {
  const frames: Array<() => void> = [];
  const result = start("/memo", "/", false, (callback) => { frames.push(callback); });
  expect(result.window.__tracktalkLastTabRedirect).toBe(true);
  expect(result.replace).not.toHaveBeenCalled();
  frames.shift()!();
  expect(result.replace).not.toHaveBeenCalled();
  frames.shift()!();
  expect(result.replace).toHaveBeenCalledExactlyOnceWith("/coaching");
});

it.each(["/memo", "/posts/123", "/?sort=new", "/?q=race", "/#section"])("直接開いたリンクを優先する: %s", (url) => {
  const result = start("/injury?tab=data", url);
  expect(result.replace).not.toHaveBeenCalled();
  expect(result.getItem).not.toHaveBeenCalled();
});

it("保存が制限されていてもコーチへ移動する", () => {
  const result = start("/memo", "/", true);
  expect(result.replace).toHaveBeenCalledExactlyOnceWith("/coaching");
  expect(result.window.__tracktalkLastTabRedirect).toBe(true);
});

it("React 側の解決も保存値にかかわらずコーチになる", () => {
  expect(resolveColdStartTarget()).toBe("/coaching");
});
