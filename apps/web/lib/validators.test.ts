import { describe, expect, it } from "vitest";
import { authErrorMessage } from "./auth-errors";
import { cloudinaryErrorMessage } from "./cloudinary-upload";
import {
  commentSchema,
  hasTextPostContent,
  inquirySchema,
  loginSchema,
  nicknameSchema,
  postSchema,
  practiceSchema,
  recruitmentSchema,
} from "./validators";

describe("validators", () => {
  it("accepts a valid post", () => {
    const parsed = postSchema.parse({
      title: "100m決勝",
      body: "展開が熱かった",
    });
    expect(parsed.title).toBe("100m決勝");
  });

  it("accepts optional text fields and requires title, body, or a photo", () => {
    expect(postSchema.safeParse({ title: "タイトルだけ", body: "" }).success).toBe(true);
    expect(postSchema.safeParse({ title: "", body: "本文だけ" }).success).toBe(true);
    expect(postSchema.safeParse({ title: "", body: "" }).success).toBe(true);
    expect(hasTextPostContent("タイトルだけ", "", false)).toBe(true);
    expect(hasTextPostContent("", "本文だけ", false)).toBe(true);
    expect(hasTextPostContent("", "", true)).toBe(true);
    expect(hasTextPostContent("  ", "\n", false)).toBe(false);
  });

  it("keeps text post length limits", () => {
    expect(postSchema.safeParse({ title: "あ".repeat(121), body: "" }).success).toBe(false);
    expect(postSchema.safeParse({ title: "", body: "あ".repeat(10001) }).success).toBe(false);
  });

  it("accepts a club recruitment and rejects a missing prefecture", () => {
    expect(
      recruitmentSchema.safeParse({
        title: "城南クラブ",
        prefectures: ["13"],
        venue: "駒沢競技場",
        schedule: "毎週火曜 19:00-21:00",
        gender_ratio: "",
        average_age: "25歳",
        recruit_age: "18〜30歳",
        links: ["https://example.com/team"],
        body: "初心者も歓迎です",
      }).success,
    ).toBe(true);
    expect(
      recruitmentSchema.safeParse({
        title: "城南クラブ",
        prefectures: [],
        venue: "",
        schedule: "毎週火曜",
        gender_ratio: "",
        average_age: "",
        recruit_age: "",
        links: [],
        body: "詳細",
      }).success,
    ).toBe(false);
  });

  it("accepts a practice partner post and rejects a blank venue", () => {
    expect(
      practiceSchema.safeParse({
        venue: "駒沢競技場",
        starts_at: "2026-10-20T18:00",
        event: "100m",
        event_other: "",
        capacity: 3,
        body: "ジョグから入ります",
      }).success,
    ).toBe(true);
    expect(
      practiceSchema.safeParse({
        venue: " ",
        starts_at: "2026-10-20T18:00",
        event: "100m",
        event_other: "",
        capacity: 0,
        body: "詳細",
      }).success,
    ).toBe(false);
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

  it("uses Japanese messages when a field is missing", () => {
    const result = loginSchema.safeParse({ email: "a@b.co" });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues[0]?.message).toBe("パスワードを入力してください");
    }
  });

  it("validates nicknames and their visibility setting", () => {
    expect(nicknameSchema.safeParse({ nickname: "ランナー", nickname_public: true }).success).toBe(true);
    expect(nicknameSchema.safeParse({ nickname: " ", nickname_public: false }).success).toBe(false);
    expect(nicknameSchema.safeParse({ nickname: "あ".repeat(10), nickname_public: false }).success).toBe(true);
    expect(nicknameSchema.safeParse({ nickname: "あ".repeat(11), nickname_public: true }).success).toBe(false);
  });

  it("translates auth and upload errors", () => {
    expect(authErrorMessage({ code: "invalid_credentials", message: "Invalid login credentials" })).toBe(
      "メールアドレスまたはパスワードが違います。",
    );
    expect(authErrorMessage({ message: "User already registered" })).toBe(
      "このメールアドレスはすでに登録されています。",
    );
    expect(cloudinaryErrorMessage("File size too large. Got 20. Maximum is 10.")).toBe(
      "ファイルサイズが大きすぎます",
    );
  });
});
