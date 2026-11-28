import { describe, expect, it } from "vitest";
import { parseYenInput } from "./coach-price";

// 本体 #1109 T1：価格入力のパース。小数は整数でない値として弾かれることを固定する
// （現行実装は数字だけの文字列以外を NaN にしており、呼び出し側が「整数で入力してください」で止める）。
describe("parseYenInput", () => {
  it("整数はそのまま数値になる", () => {
    expect(parseYenInput("1500")).toBe(1500);
    expect(parseYenInput("500")).toBe(500);
  });

  it("小数は NaN になる（1500.0 の文字列形を含む）", () => {
    expect(parseYenInput("1500.5")).toBeNaN();
    expect(parseYenInput("1500.0")).toBeNaN();
    expect(parseYenInput("1500.5円")).toBeNaN();
  });

  it("カンマ区切り・円記号・空白・全角数字は従来どおり読み替える", () => {
    expect(parseYenInput("1,500")).toBe(1500);
    expect(parseYenInput("1500円")).toBe(1500);
    expect(parseYenInput(" 1500 ")).toBe(1500);
    expect(parseYenInput("１５００")).toBe(1500);
  });

  it("500円未満は数値として返り、止めるのは呼び出し側の下限チェック", () => {
    expect(parseYenInput("499")).toBe(499);
  });

  it("空欄は null、読めない値は NaN", () => {
    expect(parseYenInput("")).toBeNull();
    expect(parseYenInput("abc")).toBeNaN();
  });
});
