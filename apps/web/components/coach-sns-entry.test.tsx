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

it("Instagram の長い URL はプラットフォーム名とハンドルだけを出し、リンク先は元の URL のまま", async () => {
  await render("https://www.instagram.com/testcoach/?utm_source=qr");
  const link = container.querySelector("a");
  expect(link?.getAttribute("href")).toBe("https://www.instagram.com/testcoach/?utm_source=qr");
  expect(link?.getAttribute("target")).toBe("_blank");
  expect(link?.textContent).toContain("Instagram");
  expect(link?.textContent).toContain("@testcoach");
  expect(link?.textContent).not.toContain("utm_source");
});

it("X と YouTube はプラットフォーム名で出す", () => {
  expect(snsLabel("https://x.com/tracktalk")?.platform).toBe("X");
  expect(snsLabel("https://x.com/tracktalk")?.short).toBe("@tracktalk");
  expect(snsLabel("https://www.youtube.com/watch?v=abc")?.platform).toBe("YouTube");
});

it("知らないホストはドメインを出し、深いパスはドメインに丸める", () => {
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
