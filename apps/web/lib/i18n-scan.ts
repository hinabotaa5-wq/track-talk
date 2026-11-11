// 画面の文言の英語辞書の網羅を調べる（本体 #530）。テスト（i18n-coverage.test.ts）と
// 辞書づくりの補助（scripts/i18n-missing.mjs）から使う。アプリ本体からは読み込まない。
import fs from "node:fs";
import path from "node:path";
import ts from "typescript";

// 日本語（かな・漢字）を含むか。
export const japanese = /[぀-ヿ㐀-鿿、。「」『』（）：！？]/;

// 英語化の対象外（運営者決定 2026-11-11、本体 #530）：管理画面（運営者専用）と規約類の本文。
// ほかに、どちらの言語でも同じに出すもの（言語選択のプロンプトは日英併記。言語名はその言語自身の表記で、行末の「// i18n-data」で外す）。
export const exemptPatterns: readonly RegExp[] = [
  /^app\/admin\//,
  /^components\/(admin-stats|announcements-admin|coaching-admin|menu-contest-admin|meet-sessions-admin|overseas-interest-admin|report-hide-actions)\.tsx$/,
  /^app\/terms\//,
  /^app\/legal\//,
  /^components\/(privacy-policy|overseas-consent-notice|support-plan-coach-terms)\.tsx$/,
  // 規約の条の見出し（第n条）。Rule は見出しを呼び出し側で訳す。
  /^components\/legal-prose\.tsx$/,
  /^components\/locale-prompt\.tsx$/,
  /^lib\/i18n(-en|-scan)?\.ts$/,
  // 画面の文言ではなくデータ：運営者の氏名（特商法の表記などに出す本名）と、ニックネームに使えない語の照合用の一覧。
  /^lib\/(operator|reserved-nickname)\.ts$/,
];

const sourceRoots = ["app", "components", "lib"];

export function listSourceFiles(root: string): string[] {
  const out: string[] = [];
  function walk(dir: string) {
    for (const entry of fs.readdirSync(path.join(root, dir), { withFileTypes: true })) {
      const rel = path.posix.join(dir, entry.name);
      if (entry.isDirectory()) walk(rel);
      else if (/\.(ts|tsx)$/.test(entry.name) && !/\.test\.(ts|tsx)$/.test(entry.name) && !/\.d\.ts$/.test(entry.name)) out.push(rel);
    }
  }
  for (const dir of sourceRoots) walk(dir);
  return out.sort();
}

export function isExempt(file: string): boolean {
  return exemptPatterns.some((pattern) => pattern.test(file));
}

export type Finding = { file: string; line: number; text: string; kind: "key" | "jsx-text" };

function isLCall(node: ts.Node | undefined): node is ts.CallExpression {
  return !!node && ts.isCallExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === "L";
}

// ファイルの中の日本語の文言を集める。
// - key：英語辞書に必要な鍵（L の第1引数、または表示に回る日本語のリテラル。タイトルは「｜」で区切った部分ごと）
// - jsx-text：L で包まれていない JSX のテキスト（そのまま日本語が出る）
export function scanSource(file: string, source: string): Finding[] {
  const sf = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, file.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS);
  const findings: Finding[] = [];
  const line = (node: ts.Node) => sf.getLineAndCharacterOfPosition(node.getStart(sf)).line + 1;
  // 行末に「// i18n-data」と書いた行の文字列は、画面の文言ではなくデータとして扱う（訳さない）。
  const dataLines = new Set(source.split("\n").flatMap((text, index) => (/\/\/\s*i18n-data\b/.test(text) ? [index + 1] : [])));
  function addKey(node: ts.Node, text: string) {
    findings.push({ file, line: line(node), text, kind: "key" });
  }
  function visit(node: ts.Node) {
    if (ts.isJsxText(node) && japanese.test(node.text)) {
      findings.push({ file, line: line(node), text: node.text.trim(), kind: "jsx-text" });
    } else if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
      const parent = node.parent;
      const skip =
        ts.isImportDeclaration(parent) ||
        ts.isLiteralTypeNode(parent) ||
        (ts.isPropertyAssignment(parent) && parent.name === node) ||
        ts.isExternalModuleReference(parent);
      if (!skip && !dataLines.has(line(node)) && japanese.test(node.text)) {
        if (isLCall(parent) && parent.arguments[0] === node) addKey(node, node.text);
        else for (const part of node.text.split("｜")) if (japanese.test(part)) addKey(node, part.trim());
      }
    } else if (ts.isTemplateExpression(node) && !dataLines.has(line(node))) {
      for (const text of [node.head.text, ...node.templateSpans.map((span) => span.literal.text)]) {
        for (const part of text.split("｜")) if (japanese.test(part)) addKey(node, part.trim());
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(sf);
  return findings;
}

export function scanAll(root: string): Finding[] {
  return listSourceFiles(root)
    .filter((file) => !isExempt(file))
    .flatMap((file) => scanSource(file, fs.readFileSync(path.join(root, file), "utf8")));
}
