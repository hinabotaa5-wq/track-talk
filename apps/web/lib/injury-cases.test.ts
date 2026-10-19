import { describe, expect, it } from "vitest";
import { injuryCaseNeedText, toggleChoice, type InjuryCaseGroup } from "./injury-cases";

describe("toggleChoice", () => {
  it("入れる・外す", () => {
    expect(toggleChoice([], "a", 2)).toEqual(["a"]);
    expect(toggleChoice(["a", "b"], "a", 2)).toEqual(["b"]);
  });
  it("上限に届いたら増やさない", () => {
    expect(toggleChoice(["a", "b"], "c", 2)).toEqual(["a", "b"]);
  });
});

describe("injuryCaseNeedText", () => {
  it("あと何人で出るかを返す", () => {
    const group = { cases: 3 } as InjuryCaseGroup;
    expect(injuryCaseNeedText(group, 5)).toBe("回答 3人（あと2人で集計を表示）");
  });
});
