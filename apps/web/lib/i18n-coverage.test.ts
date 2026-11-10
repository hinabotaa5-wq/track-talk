// 英語を選んだときに、画面のどこにも日本語が残らないことを確かめる（本体 #530）。
// 対象外（管理画面・規約類の本文など）は lib/i18n-scan.ts の exemptPatterns にまとめ、理由を書く。
import path from "node:path";
import { describe, expect, it } from "vitest";
import { en } from "./i18n-en";
import { japanese, scanAll, scanSource } from "./i18n-scan";

const root = path.resolve(__dirname, "..");
const findings = scanAll(root);

function placeholders(text: string) {
  return [...text.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort();
}

describe("英語辞書の網羅", () => {
  it("L で包まれていない日本語の JSX テキストがない", () => {
    const raw = findings.filter((f) => f.kind === "jsx-text").map((f) => `${f.file}:${f.line} ${f.text}`);
    expect(raw).toEqual([]);
  });

  it("画面の日本語の文言に、すべて英語がある", () => {
    const missing = [...new Set(findings.filter((f) => f.kind === "key" && en[f.text] === undefined).map((f) => `${f.file}:${f.line} ${f.text}`))];
    expect(missing).toEqual([]);
  });

  it("英語の文に日本語が混ざっていない", () => {
    const mixed = Object.entries(en).filter(([, value]) => japanese.test(value)).map(([key]) => key);
    expect(mixed).toEqual([]);
  });

  it("英語の文は、日本語と同じ差し込み（{name}）を持つ", () => {
    const mismatched = Object.entries(en)
      .filter(([key, value]) => placeholders(key).join() !== placeholders(value).join())
      .map(([key, value]) => `${key} → ${value}`);
    expect(mismatched).toEqual([]);
  });
});

describe("scanSource", () => {
  it("JSX テキスト・リテラル・タイトルの部分を拾い、L の鍵はそのまま使う", () => {
    const found = scanSource(
      "x.tsx",
      [
        'import { L } from "@/lib/i18n";',
        'export const metadata = { title: "ケガ部｜集まる場所" };',
        "export function X() {",
        '  return <div title={L("{n}件", { n: 1 })}>そのまま<span>{"包んでいない"}</span></div>;',
        "}",
      ].join("\n"),
    );
    expect(found.map((f) => `${f.kind}:${f.text}`)).toEqual(["key:ケガ部", "key:集まる場所", "key:{n}件", "jsx-text:そのまま", "key:包んでいない"]);
  });

  it("「// i18n-data」の行と、型・プロパティ名の日本語は拾わない", () => {
    const found = scanSource(
      "x.ts",
      ['const city = "市"; // i18n-data', 'type Noun = "写真" | "動画";', 'const map = { "東京": 1 };'].join("\n"),
    );
    expect(found).toEqual([]);
  });
});
