/** @vitest-environment jsdom */
import { afterEach, beforeEach, expect, it } from "vitest";
import { trackHeaderHistory } from "./header-history";

let tracker: ReturnType<typeof trackHeaderHistory>;
beforeEach(() => {
  window.history.replaceState({ __NA: true }, "", "/upload");
  tracker = trackHeaderHistory(window.history, window.location);
});
afterEach(() => tracker.dispose());

it("ログイン直後の履歴は対象外で、置換を繰り返しても戻れない", () => {
  expect(tracker.canGoBack(true)).toBe(false);
  window.history.replaceState({ __NA: true }, "", "/coach/apply");
  window.history.replaceState({ __NA: true }, "", "/coach");
  expect(tracker.canGoBack(true)).toBe(false);
  expect(window.history.state.__NA).toBe(true);
});

it("内部リンクで追加した履歴には戻り、置換しても戻り先は維持する", () => {
  window.history.pushState({ __NA: true }, "", "/posts/new");
  window.history.replaceState({ __NA: true }, "", "/posts/url");
  expect(tracker.canGoBack(true)).toBe(true);
  expect(window.history.state.__NA).toBe(true);
});

it("ログイン後はログイン・登録・認証ページを戻り先にしない", () => {
  for (const path of ["/login?next=/upload", "/register", "/auth/callback?code=test", "/reset-password/update"]) {
    window.history.replaceState({}, "", path);
    window.history.pushState({}, "", "/me/profile");
    expect(tracker.canGoBack(true)).toBe(false);
    expect(tracker.canGoBack(false)).toBe(true);
  }
});

it("ブラウザで戻った後の履歴を認識する", async () => {
  window.history.pushState({}, "", "/posts/new");
  await new Promise<void>((resolve) => {
    window.addEventListener("popstate", () => resolve(), { once: true });
    window.history.back();
  });
  expect(window.location.pathname).toBe("/upload");
  expect(tracker.canGoBack(true)).toBe(false);
});

it("同じURLを再度開いた場合も、実際に追加した履歴を認識する", () => {
  window.history.pushState({}, "", "/posts/new");
  window.history.pushState({}, "", "/upload");
  expect(tracker.canGoBack(true)).toBe(true);
});
