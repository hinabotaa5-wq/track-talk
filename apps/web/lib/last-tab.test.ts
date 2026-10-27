import { runInNewContext } from "node:vm";
import { expect, it, vi } from "vitest";
import { lastTabHref, lastTabStartupScript } from "./last-tab";

function start(saved: string | null, url = "/", blocked = false, requestAnimationFrame?: (callback: () => void) => void) {
  const parsed = new URL(url, "https://tracktalk.jp");
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
  ["/memo", ""], ["/coaching", ""], ["/injury", "tab=data&q=secret"],
  ["/competitions", "view=international&page=3"], ["/", "sort=review&type=spike&q=secret"],
])("React がなくても許可したタブへ即時に移動する: %s", (path, search) => {
  const result = start(`${path}?${search}`);
  expect(result.replace).toHaveBeenCalledExactlyOnceWith(lastTabHref(path, search));
  expect(result.window.__tracktalkLastTabRedirect).toBe(true);
});

it("起動ロゴを一度描画してから復元し、その間も二重遷移を防ぐ", () => {
  const frames: Array<() => void> = [];
  const result = start("/memo", "/", false, (callback) => { frames.push(callback); });
  expect(result.window.__tracktalkLastTabRedirect).toBe(true);
  expect(result.replace).not.toHaveBeenCalled();
  frames.shift()!();
  expect(result.replace).not.toHaveBeenCalled();
  frames.shift()!();
  expect(result.replace).toHaveBeenCalledExactlyOnceWith("/memo");
});

it.each([null, "/", "https://example.com", "//example.com", "/login?next=/memo", "/posts/123", "constructor", "/injury#bad"])("初回・ホーム・不正な保存値では移動しない: %s", (saved) => {
  expect(start(saved).replace).not.toHaveBeenCalled();
});

it.each(["/memo", "/posts/123", "/?sort=new", "/?q=race", "/#section"])("直接開いたリンクを優先する: %s", (url) => {
  const result = start("/injury?tab=data", url);
  expect(result.replace).not.toHaveBeenCalled();
  expect(result.getItem).not.toHaveBeenCalled();
});

it("保存が制限されていても起動を妨げない", () => {
  const result = start("/memo", "/", true);
  expect(result.replace).not.toHaveBeenCalled();
  expect(result.window.__tracktalkLastTabRedirect).toBe(false);
});
