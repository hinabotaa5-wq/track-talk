/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it } from "vitest";
import { CoachSnsEntry, snsLabel } from "./coach-sns-entry";

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

async function render(value: string) {
  await act(async () => root.render(<CoachSnsEntry value={value} />));
}

it("Instagram の URL は文字なしのアイコンリンクだけを出し、リンク先は元の URL のまま", async () => {
  await render("https://www.instagram.com/testcoach/?utm_source=qr");
  const link = container.querySelector("a");
  expect(link?.getAttribute("href")).toBe("https://www.instagram.com/testcoach/?utm_source=qr");
  expect(link?.getAttribute("target")).toBe("_blank");
  expect(link?.getAttribute("rel")).toBe("noopener noreferrer");
  expect(link?.getAttribute("aria-label")).toBe("Instagram @testcoach");
  expect(link?.getAttribute("title")).toBe("Instagram @testcoach");
  expect(link?.textContent).toBe("");
  expect(link?.querySelector("svg")).not.toBeNull();
  expect(link?.className).toContain("size-11");
  expect(link?.querySelector("svg")?.className.baseVal ?? link?.querySelector("svg")?.getAttribute("class")).toContain("size-6");
});

it("YouTube チャンネル URL はアイコンリンクになり、チャンネル ID やドメインの文字は出ない", async () => {
  await render("https://www.youtube.com/@someteam");
  const link = container.querySelector("a");
  expect(link?.getAttribute("href")).toBe("https://www.youtube.com/@someteam");
  expect(link?.getAttribute("aria-label")).toBe("YouTube @someteam");
  expect(link?.getAttribute("title")).toBe("YouTube @someteam");
  expect(link?.textContent).toBe("");
  expect(link?.textContent).not.toContain("youtube.com");
});

it("X と YouTube のプラットフォーム判定は変わらない", () => {
  expect(snsLabel("https://x.com/tracktalk")?.platform).toBe("X");
  expect(snsLabel("https://x.com/tracktalk")?.short).toBe("@tracktalk");
  expect(snsLabel("https://www.youtube.com/watch?v=abc")?.platform).toBe("YouTube");
});

it("知らないホストは汎用アイコンで出し、深いパスはドメインに丸める", async () => {
  await render("https://example.com/a/b");
  const link = container.querySelector("a");
  expect(link?.getAttribute("href")).toBe("https://example.com/a/b");
  expect(link?.getAttribute("aria-label")).toBe("example.com example.com");
  expect(link?.textContent).toBe("");
  expect(link?.querySelector("svg")).not.toBeNull();
  expect(snsLabel("https://example.com/coach")?.platform).toBe("example.com");
  expect(snsLabel("https://example.com/coach")?.short).toBe("@coach");
  expect(snsLabel("https://example.com/a/b")?.short).toBe("example.com");
});

it("URL でない入力はリンクにせずチップ風のテキストで出す", async () => {
  await render("@tracktalk.office");
  expect(container.querySelector("a")).toBeNull();
  expect(container.textContent).toContain("@tracktalk.office");
  await render("javascript:alert(1)");
  expect(container.querySelector("a")).toBeNull();
});
