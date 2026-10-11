import { describe, expect, it } from "vitest";
import { commentSchema, postSchema, profileSchema } from "./validators";

describe("validators", () => {
  it("accepts a valid post", () => {
    const parsed = postSchema.parse({
      title: "100m決勝",
      body: "展開が熱かった",
      category_id: "race_impression",
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
});
