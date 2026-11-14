// 言語切替の死キー（W-u3）の留め金。i18n-coverage.test.ts は L→en 方向だけを見るため、
// 使われなくなった t() キーは静かに残る。その検出をここで足す。
// 対象は register.language 系・profile.language 系の ja／en（6キー）。
// 言語切替UIは廃止済み：登録画面の選択は出さない（app/register/page.test.ts がUI級で担保）、
// me/language は /me へ redirect、profile は useLocale 読取専用、
// 表示言語はブラウザ自動判定が現行方針（運営者決定 2026-11-13）。
// 削除自体は製品判断（profile 初回保存の locale 送信の保全が必要）のためしない。
// このテストは「静かに増やさない・片側だけ消さない・使う側を出さずに消さない」ためのもの。
// 削除・復活のどちらの製品判断も、このテストの更新とセットで行うこと。
import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { listSourceFiles } from "./i18n-scan";

const staleLanguageKeys = [
  "register.language",
  "register.languageHint",
  "register.language.ja",
  "register.language.en",
  "profile.language",
  "profile.languageHint",
] as const;

const root = path.resolve(__dirname, "..");
const dictSource = fs.readFileSync(path.join(root, "lib", "i18n.ts"), "utf8");

function blockLines(startMarker: string, endMarker: string): string[] {
  const start = dictSource.indexOf(startMarker);
  const end = dictSource.indexOf(endMarker, start);
  if (start < 0 || end < 0) throw new Error(`dict block not found: ${startMarker}`);
  return dictSource.slice(start, end).split("\n");
}

function dictKeys(lines: string[]): Set<string> {
  const keys = new Set<string>();
  for (const line of lines) {
    const m = /^  "([^"]+)":/.exec(line);
    if (m) keys.add(m[1]);
  }
  return keys;
}

const jaKeys = dictKeys(blockLines("const ja = {", "} as const;"));
const enKeys = dictKeys(blockLines("const en: Record<I18nKey, string> = {", "\n};"));

// 辞書の定義・走査道具自体は「使う側」に数えない。
const runtimeFiles = listSourceFiles(root).filter(
  (file) => file !== "lib/i18n.ts" && file !== "lib/i18n-en.ts" && file !== "lib/i18n-scan.ts",
);

function usedIn(file: string, key: string): number[] {
  const hits: number[] = [];
  const source = fs.readFileSync(path.join(root, file), "utf8").split("\n");
  source.forEach((line, index) => {
    if (line.includes(`"${key}"`) || line.includes(`'${key}'`) || line.includes(`\`${key}\``)) hits.push(index + 1);
  });
  return hits;
}

describe("言語切替の死キー", () => {
  it("死キー6件は ja 辞書・en 辞書の両方に残っている（片側だけ消さない）", () => {
    const missing = (staleLanguageKeys as readonly string[]).flatMap((key) => [
      ...(jaKeys.has(key) ? [] : [`ja:${key}`]),
      ...(enKeys.has(key) ? [] : [`en:${key}`]),
    ]);
    expect(missing).toEqual([]);
  });

  it("死キー6件は実行時の画面・libから参照されていない（使う側を出さない）", () => {
    const refs = runtimeFiles.flatMap((file) =>
      (staleLanguageKeys as readonly string[]).flatMap((key) =>
        usedIn(file, key).map((line) => `${file}:${line} ${key}`),
      ),
    );
    expect(refs).toEqual([]);
  });
});
