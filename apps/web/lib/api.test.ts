import { describe, expect, it } from "vitest";
import { ApiRequestError, apiErrorText } from "./api";

describe("apiErrorText", () => {
  it("項目ごとの指摘があれば最初の1つを出す", () => {
    const err = new ApiRequestError(422, {
      error: { code: "validation", message: "入力を確かめてください", fields: { title: "タイトルを入れてね", body: "本文を入れてね" } },
    });
    expect(apiErrorText(err, "保存できませんでした")).toBe("タイトルを入れてね");
  });

  it("項目の指摘がなければ API のメッセージを出す", () => {
    expect(apiErrorText(new ApiRequestError(403, { error: { code: "forbidden", message: "できません" } }), "x")).toBe("できません");
    expect(apiErrorText(new ApiRequestError(422, { error: { code: "validation", message: "だめ", fields: {} } }), "x")).toBe("だめ");
  });

  it("API 以外のエラーは fallback を出す", () => {
    expect(apiErrorText(new Error("network"), "保存できませんでした")).toBe("保存できませんでした");
    expect(apiErrorText("oops", "保存できませんでした")).toBe("保存できませんでした");
  });
});
