import { currentLocale } from "./i18n";

// 英語表示のときだけ、コーチプロフィールの訳文を使う共通ヘルパー（本体 #824）。
// locale が en で、対応する *_en が空でない文字列のときだけ訳文。それ以外は原文。項目単位でフォールバックする。
export function coachDisplayText(original: string | null | undefined, translated: string | null | undefined): string | null {
  if (currentLocale() === "en" && typeof translated === "string" && translated !== "") {
    return translated;
  }
  return original ?? null;
}

// コーチの国コード（ISO 3166-1 alpha-2 の大文字2字）から国旗の絵文字を作る（本体 #932）。
// regional indicator 記号を2つ並べるだけで、画像や依存は使わない。形が合わないときは空文字（例外を投げない）。
export function countryFlagEmoji(code: string | null | undefined): string {
  if (typeof code !== "string" || !/^[A-Z]{2}$/.test(code)) return "";
  return String.fromCodePoint(...Array.from(code, (ch) => 0x1f1e6 + ch.charCodeAt(0) - 65));
}
