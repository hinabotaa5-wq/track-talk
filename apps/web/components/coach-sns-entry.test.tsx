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

function iconOf(link: Element | null | undefined) {
  const svg = link?.querySelector("svg") ?? null;
  return { svg, cls: svg?.getAttribute("class") ?? "" };
}

// lucide の Globe は円（circle）を中心とする形。ブランドグリフはどれも circle を使わない（#951）。
function isGlobe(svg: Element | null | undefined) {
  return svg?.querySelector("circle") != null && svg?.getAttribute("data-sns-icon") == null;
}

function filled(svg: Element | null, color: string) {
  return Array.from(svg?.querySelectorAll("[fill]") ?? []).some((el) => el.getAttribute("fill")?.toUpperCase() === color);
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
  const { svg, cls } = iconOf(link);
  expect(svg).not.toBeNull();
  expect(link?.className).toContain("size-11");
  // A1：現行の線画グリフの形のまま、線の色が #E1306C、大きさは size-7（#951）。
  expect(cls).toContain("size-7");
  expect(cls).not.toContain("size-6");
  expect(svg?.getAttribute("stroke")?.toUpperCase()).toBe("#E1306C");
  expect(svg?.querySelector('rect[rx="5"]')).not.toBeNull();
  expect(svg?.querySelector("path")?.getAttribute("d")).toContain("11.37");
  expect(isGlobe(svg)).toBe(false);
  expect(svg?.querySelector("circle")).toBeNull();
});

it.each([
  ["https://www.youtube.com/@someteam", "YouTube @someteam"],
  ["https://youtu.be/abc123", "YouTube @abc123"],
])("YouTube（%s）は Globe ではなく赤い角丸長方形＋白い三角の YouTube グリフを size-7 で出す", async (url, name) => {
  await render(url);
  const link = container.querySelector("a");
  expect(link?.getAttribute("href")).toBe(url);
  expect(link?.getAttribute("aria-label")).toBe(name);
  expect(link?.getAttribute("title")).toBe(name);
  expect(link?.textContent).toBe("");
  expect(link?.textContent).not.toContain("youtube.com");
  expect(link?.className).toContain("size-11");
  const { svg, cls } = iconOf(link);
  expect(cls).toContain("size-7");
  expect(isGlobe(svg)).toBe(false);
  expect(svg?.querySelector("circle")).toBeNull();
  // A2：赤 #FF0000 の角丸長方形と、白い再生三角。
  const rect = svg?.querySelector("rect");
  expect(rect?.getAttribute("fill")?.toUpperCase()).toBe("#FF0000");
  expect(Number(rect?.getAttribute("rx"))).toBeGreaterThan(0);
  expect(filled(svg, "#FFFFFF")).toBe(true);
});

const brandCases: [string, string, string, string][] = [
  ["https://x.com/tracktalk", "X", "X @tracktalk", "#0F172A"],
  ["https://twitter.com/tracktalk", "X", "X @tracktalk", "#0F172A"],
  ["https://www.tiktok.com/@tracktalk", "TikTok", "TikTok @tracktalk", "#0F172A"],
  ["https://www.facebook.com/tracktalk", "Facebook", "Facebook @tracktalk", "#1877F2"],
  ["https://line.me/R/ti/p/@tracktalk", "LINE", "LINE line.me", "#06C755"],
  ["https://note.com/tracktalk?x=1", "note", "note @tracktalk", "#0F172A"],
];

it.each(brandCases)("%s は %s 専用のブランド色グリフ（Globe でない）を size-7 で出し、リンクの属性は変わらない", async (url, platform, name, color) => {
  await render(url);
  const link = container.querySelector("a");
  // C1：href（クエリ込み）・target・rel・aria-label・title は入力どおり、テキストは空。
  expect(link?.getAttribute("href")).toBe(new URL(url).href);
  expect(link?.getAttribute("target")).toBe("_blank");
  expect(link?.getAttribute("rel")).toBe("noopener noreferrer");
  expect(link?.getAttribute("aria-label")).toBe(name);
  expect(link?.getAttribute("title")).toBe(name);
  expect(link?.textContent).toBe("");
  expect(link?.className).toContain("size-11");
  const { svg, cls } = iconOf(link);
  expect(cls).toContain("size-7");
  expect(svg?.getAttribute("data-sns-icon")).toBe(platform);
  expect(isGlobe(svg)).toBe(false);
  expect(svg?.querySelector("circle")).toBeNull();
  // A3：§2 のブランド色を持つ。
  expect(filled(svg, color)).toBe(true);
});

it("7プラットフォームのグリフはそれぞれ異なる SVG で、note と LINE は白抜きの字形を持つ", async () => {
  const urls = [
    "https://www.instagram.com/a",
    "https://www.youtube.com/@a",
    "https://x.com/a",
    "https://www.tiktok.com/@a",
    "https://www.facebook.com/a",
    "https://line.me/R/ti/p/@a",
    "https://note.com/a",
  ];
  const markups = new Set<string>();
  for (const url of urls) {
    await render(url);
    const { svg } = iconOf(container.querySelector("a"));
    markups.add(svg?.outerHTML ?? "");
    if (/line\.me|note\.com/.test(url)) {
      expect(filled(svg, "#FFFFFF")).toBe(true);
      // 字形はパスで描く（<text> は使わない）。
      expect(svg?.querySelector("text")).toBeNull();
    }
  }
  expect(markups.size).toBe(urls.length);
});

it("X と YouTube のプラットフォーム判定は変わらない", () => {
  expect(snsLabel("https://x.com/tracktalk")?.platform).toBe("X");
  expect(snsLabel("https://x.com/tracktalk")?.short).toBe("@tracktalk");
  expect(snsLabel("https://www.youtube.com/watch?v=abc")?.platform).toBe("YouTube");
});

it("全プラットフォームの判定（platform／short）は変わらない", () => {
  const cases: [string, string, string][] = [
    ["https://instagram.com/a", "Instagram", "@a"],
    ["https://m.instagram.com/a/", "Instagram", "@a"],
    ["https://twitter.com/a", "X", "@a"],
    ["https://m.youtube.com/@a", "YouTube", "@a"],
    ["https://youtu.be/abc", "YouTube", "@abc"],
    ["https://www.tiktok.com/@a", "TikTok", "@a"],
    ["https://facebook.com/a", "Facebook", "@a"],
    ["https://line.me/R/ti/p/@a", "LINE", "line.me"],
    ["https://www.note.com/a", "note", "@a"],
  ];
  for (const [url, platform, short] of cases) {
    expect(snsLabel(url)).toEqual({ platform, short, href: new URL(url).href });
  }
});

it("知らないホストは汎用アイコンで出し、深いパスはドメインに丸める", async () => {
  await render("https://example.com/a/b");
  const link = container.querySelector("a");
  expect(link?.getAttribute("href")).toBe("https://example.com/a/b");
  expect(link?.getAttribute("aria-label")).toBe("example.com example.com");
  expect(link?.textContent).toBe("");
  const svg = link?.querySelector("svg");
  expect(svg).not.toBeNull();
  // A4：未知ホストは従来どおり Globe（Instagram グリフになっていないこと）で、大きさは size-7。
  expect(isGlobe(svg)).toBe(true);
  expect(svg?.getAttribute("class")).toContain("size-7");
  expect(link?.className).toContain("text-slate-200");
  expect(svg?.querySelector('rect[rx="5"]')).toBeNull();
  expect(snsLabel("https://example.com/coach")?.platform).toBe("example.com");
  expect(snsLabel("https://example.com/coach")?.short).toBe("@coach");
  expect(snsLabel("https://example.com/a/b")?.short).toBe("example.com");
});

it("URL でない入力はリンクにせずチップ風のテキストで出す", async () => {
  await render("@tracktalk.office");
  expect(container.querySelector("a")).toBeNull();
  expect(container.textContent).toContain("@tracktalk.office");
  expect(container.querySelector("span")?.className).toContain("rounded-full");
  await render("javascript:alert(1)");
  expect(container.querySelector("a")).toBeNull();
  expect(container.textContent).toContain("javascript:alert(1)");
});
