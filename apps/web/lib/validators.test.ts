import { describe, expect, it } from "vitest";
import { authErrorMessage } from "./auth-errors";
import { cloudinaryErrorMessage } from "./cloudinary-upload";
import {
  changeEmailSchema,
  changePasswordSchema,
  commentSchema,
  hasTextPostContent,
  inquirySchema,
  isYouTubeUrl,
  loginSchema,
  nicknameSchema,
  postSchema,
  practiceSchema,
  recruitmentSchema,
} from "./validators";

describe("validators", () => {
  it("accepts YouTube watch, short link and Shorts URLs", () => {
    expect(isYouTubeUrl("https://www.youtube.com/watch?v=abcdefghijk")).toBe(true);
    expect(isYouTubeUrl("https://youtu.be/abcdefghijk")).toBe(true);
    expect(isYouTubeUrl("https://www.youtube.com/shorts/abcdefghijk")).toBe(true);
    expect(isYouTubeUrl("https://youtube.com/shorts/abcdefghijk?feature=share")).toBe(true);
    expect(isYouTubeUrl("https://m.youtube.com/shorts/abcdefghijk")).toBe(true);
  });

  it("rejects non-YouTube URLs and bare Shorts paths", () => {
    expect(isYouTubeUrl("https://example.com/shorts/abcdefghijk")).toBe(false);
    expect(isYouTubeUrl("https://www.youtube.com/shorts/")).toBe(false);
    expect(isYouTubeUrl("https://www.youtube.com/")).toBe(false);
  });

  it("accepts a valid post", () => {
    const parsed = postSchema.parse({
      title: "100m決勝",
      body: "展開が熱かった",
    });
    expect(parsed.title).toBe("100m決勝");
  });

  it("accepts optional text fields and requires a title", () => {
    expect(postSchema.safeParse({ title: "タイトルだけ", body: "" }).success).toBe(true);
    expect(postSchema.safeParse({ title: "", body: "本文だけ" }).success).toBe(false); // タイトル必須化により false
    expect(postSchema.safeParse({ title: "", body: "" }).success).toBe(false); // タイトル必須化により false
    expect(hasTextPostContent("タイトルだけ")).toBe(true);
    expect(hasTextPostContent("")).toBe(false);
    expect(hasTextPostContent("  ")).toBe(false);
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
      inquirySchema.safeParse({ category: "bug", email: "a@b.co", subject: "ログインできない", body: "再設定後も入れません" }).success,
    ).toBe(true);
    expect(inquirySchema.safeParse({ category: "bug", email: "a@b.co", subject: "  ", body: "本文" }).success).toBe(false);
    expect(inquirySchema.safeParse({ category: "bug", email: "", subject: "件名", body: "本文" }).success).toBe(false);
    expect(inquirySchema.safeParse({ category: "bug", email: "not-an-email", subject: "件名", body: "本文" }).success).toBe(false);
  });

  it("uses Japanese messages when a field is missing", () => {
    const result = loginSchema.safeParse({ email: "a@b.co" });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues[0]?.message).toBe("パスワードを入力してください");
    }
  });

  it("validates nicknames and their visibility setting", () => {
    expect(nicknameSchema.safeParse({ nickname: "ランナー", nickname_public: true, event: "100m" }).success).toBe(true);
    expect(nicknameSchema.safeParse({ nickname: " ", nickname_public: false, event: "100m" }).success).toBe(false);
    expect(nicknameSchema.safeParse({ nickname: "あ".repeat(10), nickname_public: false, event: "5000m" }).success).toBe(true);
    expect(nicknameSchema.safeParse({ nickname: "あ".repeat(11), nickname_public: true, event: "5000m" }).success).toBe(false);
    expect(nicknameSchema.safeParse({ nickname: "ランナー", nickname_public: true }).success).toBe(false);
    // 種目は、コーチの担当種目と同じ一覧(リレーも選べる)。以前の区分や「その他」は選べない。
    expect(nicknameSchema.safeParse({ nickname: "ランナー", nickname_public: true, event: "relay" }).success).toBe(true);
    expect(nicknameSchema.safeParse({ nickname: "ランナー", nickname_public: true, event: "sprint" }).success).toBe(false);
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

describe("account change schemas", () => {
  it("requires the current password and a matching new password", () => {
    expect(changePasswordSchema.safeParse({ current: "oldpassword", password: "newpassword", confirm: "newpassword" }).success).toBe(true);
    expect(changePasswordSchema.safeParse({ current: "", password: "newpassword", confirm: "newpassword" }).success).toBe(false);
    expect(changePasswordSchema.safeParse({ current: "oldpassword", password: "short", confirm: "short" }).success).toBe(false);
    expect(changePasswordSchema.safeParse({ current: "oldpassword", password: "newpassword", confirm: "different1" }).success).toBe(false);
  });

  it("checks the new email address", () => {
    expect(changeEmailSchema.safeParse({ email: "new@example.com", current: "oldpassword" }).success).toBe(true);
    expect(changeEmailSchema.safeParse({ email: "not-an-email", current: "oldpassword" }).success).toBe(false);
    expect(changeEmailSchema.safeParse({ email: "new@example.com", current: "" }).success).toBe(false);
  });
});
