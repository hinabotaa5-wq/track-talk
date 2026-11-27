/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import { CoachOfferSections } from "./coach-offer-view";

// コーチ詳細の「このコーチの添削で得られるもの」と見本動画（本体 #1086 W3・W6）。
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });

function titles() {
  return [...container.querySelectorAll('[data-testid="coach-offer-items"] li')].map((li) => li.textContent);
}

it("W3: タイトルだけを固定4項目→追加項目（登録順）で並べ、見本動画は別の節で再生できる", async () => {
  await act(async () => root.render(<CoachOfferSections items={["スタート局面の詳しい分析", "後半の失速の分析"]} videos={[{ url: "https://example.com/a.mp4" }, { url: "https://example.com/b.mp4" }]} />));
  expect(container.querySelector("h2")?.textContent).toBe("このコーチの添削で得られるもの");
  expect(titles()).toEqual(["良い点", "改善点", "おすすめのドリル", "コーチなら明日から何をするか", "スタート局面の詳しい分析", "後半の失速の分析"]);
  const videos = [...container.querySelectorAll('[data-testid="coach-demo-videos"] video')];
  expect(videos.map((v) => v.getAttribute("src"))).toEqual(["https://example.com/a.mp4", "https://example.com/b.mp4"]);
  expect(videos.every((v) => v.hasAttribute("controls"))).toBe(true);
  // 見本動画は「得られるもの」に混ぜない。
  expect(container.querySelector('[data-testid="coach-offer-items"] video')).toBeNull();
});

it("W3・A7: 追加項目も見本もないコーチは固定4項目だけで、見本動画の節を出さない", async () => {
  await act(async () => root.render(<CoachOfferSections items={[]} videos={[]} />));
  expect(titles()).toEqual(["良い点", "改善点", "おすすめのドリル", "コーチなら明日から何をするか"]);
  expect(container.textContent).not.toContain("見本動画");
  await act(async () => root.render(<CoachOfferSections />));
  expect(titles()).toHaveLength(4);
});

it("W6: 英語表示では見出しと固定項目が英語、追加項目のタイトルは原文のまま", async () => {
  installEnglish(en);
  setActiveLocale("en");
  await act(async () => root.render(<CoachOfferSections items={["スタート局面の詳しい分析"]} videos={[{ url: "https://example.com/a.mp4" }]} />));
  expect(container.textContent).toContain("What you get from this coach's review");
  expect(container.textContent).toContain("Demo video");
  expect(titles()).toEqual(["Strong point", "What to improve", "Recommended drill", "What I would do starting tomorrow if I were you", "スタート局面の詳しい分析"]);
});
