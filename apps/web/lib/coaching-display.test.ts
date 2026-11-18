import { afterEach, describe, expect, it } from "vitest";
import { athleteText, coachMenu, coachText, coachTexts } from "./coaching-display";
import { setActiveLocale } from "./i18n";
import type { PlanMenu } from "./types";

// 海外コーチとのやり取りの訳の出し方（本体 #916、テスト細目 B1〜B4 の判定部分）。
afterEach(() => setActiveLocale("ja"));

describe("athleteText（選手側の文）", () => {
  it("表示言語が en で英訳があれば、英訳を主に原文を併記する", () => {
    expect(athleteText("出だしで腰が落ちます", "My hips drop", { mine: false, locale: "en" })).toEqual({ text: "My hips drop", original: "出だしで腰が落ちます" });
  });
  it("表示言語が ja、英訳が無い・空、自分の文なら原文だけ", () => {
    expect(athleteText("原文", "EN", { mine: false, locale: "ja" })).toEqual({ text: "原文", original: null });
    expect(athleteText("原文", null, { mine: false, locale: "en" })).toEqual({ text: "原文", original: null });
    expect(athleteText("原文", "  ", { mine: false, locale: "en" })).toEqual({ text: "原文", original: null });
    expect(athleteText("原文", "EN", { mine: true, locale: "en" })).toEqual({ text: "原文", original: null });
  });
  it("locale を渡さなければ、いまの表示言語を使う", () => {
    setActiveLocale("en");
    expect(athleteText("原文", "EN", { mine: false }).text).toBe("EN");
  });
});

describe("coachText（コーチの文）", () => {
  it("表示言語が ja で和訳があれば、和訳を主に原文を併記する", () => {
    expect(coachText("Keep your hips high", "腰を高く保ちましょう", { mine: false, locale: "ja" })).toEqual({ text: "腰を高く保ちましょう", original: "Keep your hips high" });
  });
  it("表示言語が en、和訳が無い、自分の文なら原文だけ", () => {
    expect(coachText("Original", "訳", { mine: false, locale: "en" })).toEqual({ text: "Original", original: null });
    expect(coachText("Original", undefined, { mine: false, locale: "ja" })).toEqual({ text: "Original", original: null });
    expect(coachText("Original", "訳", { mine: true, locale: "ja" })).toEqual({ text: "Original", original: null });
  });
  it("直す点は要素ごとに判定し、数が合わない訳は使わない", () => {
    expect(coachTexts(["a", "b"], ["訳a", ""], { mine: false, locale: "ja" })).toEqual([{ text: "訳a", original: "a" }, { text: "b", original: null }]);
    expect(coachTexts(["a", "b"], ["訳a"], { mine: false, locale: "ja" })).toEqual([{ text: "a", original: null }, { text: "b", original: null }]);
  });
});

describe("coachMenu（コーチのメニュー）", () => {
  const menu: PlanMenu = { title: "Week 1", period: "12/1-12/7", days: [{ day: "Mon", content: "Strides x5" }, { day: "Wed", content: "Rest" }] };
  const ja: PlanMenu = { title: "1週目", period: "", days: [{ day: "月", content: "流し5本" }, { day: "水", content: "休み" }] };
  it("和訳を主に、構造（日ごとの並び）は原文どおりにし、原文のメニューを併記する。空の訳の項目は原文", () => {
    const shown = coachMenu(menu, ja, { mine: false, locale: "ja" });
    expect(shown.menu).toEqual({ title: "1週目", period: "12/1-12/7", days: [{ day: "月", content: "流し5本" }, { day: "水", content: "休み" }] });
    expect(shown.original).toBe(menu);
  });
  it("日の数が合わない・訳が無い・表示言語が en・自分のメニューなら原文だけ", () => {
    expect(coachMenu(menu, { ...ja, days: ja.days.slice(0, 1) }, { mine: false, locale: "ja" })).toEqual({ menu, original: null });
    expect(coachMenu(menu, null, { mine: false, locale: "ja" })).toEqual({ menu, original: null });
    expect(coachMenu(menu, ja, { mine: false, locale: "en" })).toEqual({ menu, original: null });
    expect(coachMenu(menu, ja, { mine: true, locale: "ja" })).toEqual({ menu, original: null });
  });
});
