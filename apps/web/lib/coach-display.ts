import { currentLocale } from "./i18n";

// 英語表示のときだけ、コーチプロフィールの訳文を使う共通ヘルパー（本体 #824）。
// locale が en で、対応する *_en が空でない文字列のときだけ訳文。それ以外は原文。項目単位でフォールバックする。
export function coachDisplayText(original: string | null | undefined, translated: string | null | undefined): string | null {
  if (currentLocale() === "en" && typeof translated === "string" && translated !== "") {
    return translated;
  }
  return original ?? null;
}
