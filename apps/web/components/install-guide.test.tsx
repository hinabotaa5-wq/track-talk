/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import { InstallInstructions } from "./install-guide";

vi.mock("next/navigation", () => ({ usePathname: () => "/me/install" }));

const iphoneSafari =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1";
const iphoneChrome =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/130.0 Mobile/15E148 Safari/604.1";
const android =
  "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Mobile Safari/537.36";

let container: HTMLDivElement;
let root: Root;

function setUserAgent(ua: string) {
  Object.defineProperty(window.navigator, "userAgent", { configurable: true, get: () => ua });
  Object.defineProperty(window.navigator, "maxTouchPoints", { configurable: true, get: () => 5 });
}

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  window.matchMedia = vi.fn().mockReturnValue({
    matches: false,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }) as typeof window.matchMedia;
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  setActiveLocale("ja");
  vi.unstubAllGlobals();
});

async function render() {
  await act(async () => root.render(<InstallInstructions />));
}

function svgTexts(index: number) {
  const svg = container.querySelectorAll("svg")[index];
  return [...(svg?.querySelectorAll("text") ?? [])].map((node) => node.textContent ?? "");
}

it("ios-safari は訂正後の手順を表示し、旧手順1が残らない", async () => {
  setUserAgent(iphoneSafari);
  await render();
  const text = container.textContent ?? "";
  expect(text).toContain("「共有」を開く");
  expect(text).toContain("画面下のアドレスバー左にある「≡」をタップし、出てきたメニューの一番上の「共有」をタップします。画面下に共有マーク（四角から上に出る矢印）が見える場合は、そのマークでも開けます。");
  expect(text).toContain("「ホーム画面に追加」を選ぶ");
  expect(text).toContain("一覧の「ページを検索」の下にあります。右側の「＋」マークが目印です。");
  expect(text).toContain("右上の「追加」をタップ");
  expect(text).toContain("ホーム画面に TrackTalk のアイコンができます。");
  expect(text).not.toContain("画面下の共有ボタンをタップ");
  expect(text).not.toContain("四角から矢印が出ているマークです。");
  expect(text).not.toContain("メニューを下にスクロールすると見つかります。");
});

it("手順1・2のイラストが実際の Safari の並びを示す", async () => {
  setUserAgent(iphoneSafari);
  await render();
  const shareArt = svgTexts(0);
  expect(shareArt).toContain("≡");
  expect(shareArt).toContain("共有");
  expect(shareArt).toContain("ブックマークに追加");
  expect(container.querySelectorAll("svg")[0]?.getAttribute("aria-label")).toBe(
    "アドレスバー左の「≡」をタップし、メニューの一番上の「共有」をタップ",
  );

  const sheetArt = svgTexts(1);
  const order = ["ブックマークの追加先…", "お気に入りに追加", "ページを検索", "ホーム画面に追加", "マークアップ", "プリント"];
  // DOM 順ではなく、描画位置（y 座標）の並びで確認する。
  const ys = new Map(
    [...container.querySelectorAll("svg")[1]!.querySelectorAll("text")].map((node) => [node.textContent, Number(node.getAttribute("y"))]),
  );
  const positions = order.map((label) => ys.get(label) ?? Number.NaN);
  expect(positions.every((position) => Number.isFinite(position))).toBe(true);
  expect([...positions].sort((a, b) => a - b)).toEqual(positions);
  for (const label of order) expect(sheetArt).toContain(label);
  expect(sheetArt).not.toContain("コピー");
  expect(sheetArt).not.toContain("ブックマークを追加");
});

it("ios-other は先頭が「まず Safari で開く」で、後続が訂正後の手順と同一になる", async () => {
  setUserAgent(iphoneChrome);
  await render();
  const titles = [...container.querySelectorAll("li p.font-semibold")].map((node) => node.textContent);
  expect(titles).toEqual(["まず Safari で開く", "「共有」を開く", "「ホーム画面に追加」を選ぶ", "右上の「追加」をタップ"]);
});

it("英語表示では訂正後の手順が英語になり、未訳の日本語が残らない", async () => {
  installEnglish(en);
  setActiveLocale("en");
  setUserAgent(iphoneSafari);
  await render();
  const text = container.textContent ?? "";
  expect(text).toContain("Open \"Share\"");
  expect(text).toContain("Choose \"Add to Home Screen\"");
  expect(text).toContain("Tap \"Add\" in the top right");
  expect(svgTexts(0)).toContain("Share");
  expect(svgTexts(1)).toContain("Find on Page");
  expect(text).not.toMatch(/[ぁ-んァ-ヶ一-龠]/);
});

it("Android の手順文面・イラストは従来どおり", async () => {
  setUserAgent(android);
  await render();
  const text = container.textContent ?? "";
  expect(text).toContain("右上のメニュー(︙)をタップ");
  expect(text).toContain("Chrome の画面右上にあります。");
  expect(text).toContain("「ホーム画面に追加」を選ぶ");
  expect(text).toContain("「アプリをインストール」と表示される場合もあります。");
  expect(svgTexts(1)).toContain("ホーム画面に追加");
});
