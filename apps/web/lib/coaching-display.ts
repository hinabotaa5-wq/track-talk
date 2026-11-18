// 海外コーチとのやり取りの訳の出し方（本体 #916）。
// API は、海外コーチ（locale = en）のペアで訳が新しいときだけ *_en／*_ja を返し、それ以外は null を返す。
// ここでは見る人の表示言語と「自分の書いた文か」で、訳を主に・原文を直下に小さく併記するかを項目ごとに決める。
// - 選手側（選手・保護者）が書いた文：表示言語が en で *_en が空でないとき、英訳を主に原文を併記。
// - コーチが書いた文：表示言語が ja で *_ja が空でないとき、和訳を主に原文を併記。
// - 自分の書いた文は常に原文だけ。
import { currentLocale, type Locale } from "./i18n";
import type { PlanMenu } from "./types";

export type DisplayedText = {
  // 主に出す文（訳があれば訳、なければ原文）。
  text: string;
  // 併記する原文。訳を出さないときは null。
  original: string | null;
};

type Options = { mine: boolean; locale?: Locale };

function pick(original: string, translated: string | null | undefined, want: boolean): DisplayedText {
  if (want && typeof translated === "string" && translated.trim() !== "") return { text: translated, original };
  return { text: original, original: null };
}

// 選手側（選手・保護者）が書いた文。
export function athleteText(original: string, en: string | null | undefined, { mine, locale = currentLocale() }: Options): DisplayedText {
  return pick(original, en, !mine && locale === "en");
}

// コーチが書いた文。
export function coachText(original: string, ja: string | null | undefined, { mine, locale = currentLocale() }: Options): DisplayedText {
  return pick(original, ja, !mine && locale === "ja");
}

// コーチの直す点（配列）。要素ごとに判定し、数が原文と合わない訳は使わない。
export function coachTexts(originals: string[], ja: string[] | null | undefined, options: Options): DisplayedText[] {
  const usable = Array.isArray(ja) && ja.length === originals.length ? ja : null;
  return originals.map((o, i) => coachText(o, usable ? usable[i] : null, options));
}

// コーチのメニュー。項目ごとに訳があれば訳を使い、1つでも訳を使ったら原文のメニュー全体を併記する。
// 日の数が原文と合わない訳は使わない。構造（日ごとの並び）は原文どおり。
export function coachMenu(menu: PlanMenu, ja: PlanMenu | null | undefined, { mine, locale = currentLocale() }: Options): { menu: PlanMenu; original: PlanMenu | null } {
  if (mine || locale !== "ja" || !ja || !Array.isArray(ja.days) || ja.days.length !== menu.days.length) return { menu, original: null };
  let used = false;
  const mark = (o: string | undefined, t: string | undefined) => {
    if (!o || !t || t.trim() === "") return o;
    used = true;
    return t;
  };
  const shown: PlanMenu = {
    title: mark(menu.title, ja.title) ?? menu.title,
    days: menu.days.map((d, i) => ({ day: mark(d.day, ja.days[i]?.day) ?? d.day, content: mark(d.content, ja.days[i]?.content) ?? d.content })),
  };
  if (menu.period !== undefined) shown.period = mark(menu.period, ja.period);
  if (menu.note !== undefined) shown.note = mark(menu.note, ja.note);
  return used ? { menu: shown, original: menu } : { menu, original: null };
}

// 併記する原文の見た目（直下に小さく）。各画面で同じクラスを使う。
export const originalTextClass = "mt-1 whitespace-pre-wrap break-words border-l-2 border-white/20 pl-2 text-xs text-slate-400";
