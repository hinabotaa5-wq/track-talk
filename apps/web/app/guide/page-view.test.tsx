/** @vitest-environment jsdom */
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import { GuideView } from "./page-view";
vi.mock("next/link", () => ({ default: ({ href, children, className }: { href: string; children: ReactNode; className?: string }) => <a href={href} className={className}>{children}</a> }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn(), replace: vi.fn() }), usePathname: () => "/", useSearchParams: () => new URLSearchParams() }));

// 本体 #879：利用ガイド「メモ」の段落に、継続プラン共有の例外文を加える。例外の対象は感覚メモだけ。
afterEach(() => setActiveLocale("ja"));

const JA_HEAD = "練習で感じたことは「感覚」に、競技会の結果は「記録」に残します。メモや記録の内容は本人だけが見られます。";
const JA_EXCEPTION = "ただし、継続プランの契約期間中は、選手が共有をやめていない感覚メモを担当コーチが見ることができます。";
const JA_REST = "自己ベストを更新した人数は、個人がわからない形で集計し、5人以上の種目だけログインした利用者に表示します。感覚メモを書くとビンに玉がたまり、練習の積み重ねを振り返れます。感覚と記録の両タブで同じカレンダーを使い、感覚メモは青、記録は黄色の印で表示します。日付を押すと、その日の感覚と記録をまとめて確認できます。記録は同じ日・種目でも1本ずつ残せます。ラウンドは任意の自由記入で、記録会などは空欄のまま保存できます。グラフは種目ごとに切り替えられ、「全種目」では各種目のグラフを小さく並べて表示します。グラフの点を押すと、その記録の日付が出ます。";
const EN_EXCEPTION = "However, during an Ongoing Plan contract, the coach in charge can see the sense notes you have not stopped sharing.";

function memoParagraph(html: string): string {
  const doc = new DOMParser().parseFromString(html, "text/html");
  const p = [...doc.querySelectorAll("p")].find((el) => (el.textContent ?? "").includes("Only you can see your notes and records.") || (el.textContent ?? "").includes("メモや記録の内容は本人だけが見られます。"));
  expect(p).toBeDefined();
  return p?.textContent ?? "";
}

it("日本語：従来文を残し、例外文を直後に置き、残りの文は従来どおり", () => {
  setActiveLocale("ja");
  const para = memoParagraph(renderToStaticMarkup(<GuideView />));
  expect(para).toContain("メモや記録の内容は本人だけが見られます。");
  expect(para).toContain(JA_EXCEPTION);
  expect(para).toBe(JA_HEAD + JA_EXCEPTION + JA_REST);
  // 例外の対象は感覚メモだけ。記録は含めない。
  expect(JA_EXCEPTION).not.toContain("記録");
});

it("英語：従来文と固定の例外文が入り、日本語の例外文は混ざらない", () => {
  installEnglish(en);
  setActiveLocale("en");
  const para = memoParagraph(renderToStaticMarkup(<GuideView />));
  expect(para).toContain("Only you can see your notes and records.");
  expect(para).toContain(EN_EXCEPTION);
  expect(para.indexOf(EN_EXCEPTION)).toBe(para.indexOf("Only you can see your notes and records.") + "Only you can see your notes and records. ".length);
  expect(para).not.toContain(JA_EXCEPTION);
  expect(para).not.toMatch(/[぀-ヿ一-鿿]/);
});

it("辞書：旧い段落キーは無く、新しい段落キーが1件だけある", () => {
  const keys = Object.keys(en).filter((k) => k.includes("メモや記録の内容は本人だけが見られます。"));
  expect(keys).toEqual([JA_HEAD + JA_EXCEPTION + JA_REST]);
  expect(en[keys[0]]).toContain(EN_EXCEPTION);
  expect(Object.keys(en)).not.toContain(JA_HEAD + JA_REST);
});
