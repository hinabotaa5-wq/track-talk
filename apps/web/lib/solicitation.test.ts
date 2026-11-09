import { expect, it } from "vitest";
import { looksLikeSolicitation } from "./solicitation";

// API の contact_test.go の warned / allowed と同じ例。
it("勧誘に使われやすい言葉を見つける（字形の違い・見えない文字もそろえる）", () => {
  for (const s of ["LINE で送ってください", "ｌｉｎｅ交換しましょう", "DMください", "インスタで動画を見せて", "ins​tagram で", "ライン交換しよ", "らいんで話そう", "Facebook で探して", "直接連絡してください", "個人レッスンもやっています", "外でやりとりしませんか", "連絡先を教えて"]) {
    expect(looksLikeSolicitation(s), s).toBe(true);
  }
});

it("陸上の用語や記録では警告しない", () => {
  for (const s of ["スタートラインに手をつく位置を、指1本ぶん前にしましょう", "ラインを意識して、真っすぐ走りましょう", "フィニッシュラインまで走り切る", "400m×10 (68 68 67 67 66)", "1500m 4:05 (61-66-67-71)"]) {
    expect(looksLikeSolicitation(s), s).toBe(false);
  }
});
