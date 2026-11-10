// 英語辞書（lib/i18n-en.ts）に無い日本語の文言を JSON で出す（本体 #530 の辞書づくりの補助）。
// usage: node --experimental-strip-types scripts/i18n-missing.mjs > missing.json
import path from "node:path";
import { fileURLToPath } from "node:url";
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const { scanAll } = await import(path.join(root, "lib/i18n-scan.ts"));
const { en } = await import(path.join(root, "lib/i18n-en.ts"));
const missing = new Map();
const jsx = [];
for (const f of scanAll(root)) {
  if (f.kind === "jsx-text") jsx.push(`${f.file}:${f.line} ${f.text}`);
  else if (en[f.text] === undefined && !missing.has(f.text)) missing.set(f.text, `${f.file}:${f.line}`);
}
console.log(JSON.stringify({ jsx, missing: Object.fromEntries(missing) }, null, 1));
