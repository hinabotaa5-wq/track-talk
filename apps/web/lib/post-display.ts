import { currentLocale } from "./i18n";

export function postDisplayText(original: string | null | undefined, translated: string | null | undefined): string | null {
  if (currentLocale() === "en" && typeof translated === "string" && translated !== "") return translated;
  return original ?? null;
}
