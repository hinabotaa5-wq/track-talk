import { describe, expect, it } from "vitest";
import { commentSchema, inquirySchema, postSchema, profileSchema } from "./validators";

describe("validators", () => {
  it("accepts a valid post", () => {
    const parsed = postSchema.parse({
      title: "100m決勝",
      body: "展開が熱かった",
    });
    expect(parsed.title).toBe("100m決勝");
  });

  it("rejects empty nickname", () => {
    const r = profileSchema.safeParse({ nickname: "a", bio: "" });
    expect(r.success).toBe(false);
  });

  it("rejects long comments", () => {
    const r = commentSchema.safeParse({ body: "あ".repeat(2001) });
    expect(r.success).toBe(false);
  });

  it("accepts an inquiry and rejects a blank subject", () => {
    expect(
      inquirySchema.safeParse({ category: "bug", subject: "ログインできない", body: "再設定後も入れません" }).success,
    ).toBe(true);
    expect(inquirySchema.safeParse({ category: "bug", subject: "  ", body: "本文" }).success).toBe(false);
  });
});
