import { z } from "zod";
import { practiceEventValues } from "./practice-events";

z.config(z.locales.ja());

export const postSchema = z.object({
  title: z.string("タイトルを入力してください").trim().min(1, "タイトルを入力してください").max(120, "120文字以内"),
  body: z.string("本文を入力してください").trim().max(10000, "10000文字以内"),
});

export function hasTextPostContent(title: string) {
  return title.trim() !== "";
}

export function isYouTubeUrl(value: string) {
  return /^https?:\/\/(((www\.|m\.)?youtube\.com\/(watch\?v=|shorts\/))|youtu\.be\/)[^\s]+$/i.test(value.trim());
}

function httpsUrl(value: string) {
  try {
    const url = new URL(value);
    return url.protocol === "https:" && url.hostname !== "";
  } catch {
    return false;
  }
}

export const recruitmentSchema = z.object({
  title: z.string("チーム名を入力してください").trim().min(1, "チーム名を入力してください").max(120, "120文字以内"),
  prefectures: z.array(z.string()).min(1, "都道府県を1つ以上選んでください"),
  venue: z.string().trim().max(80, "活動場所は80文字以内で入力してください"),
  schedule: z.string("活動時間を入力してください").trim().min(1, "活動時間を入力してください").max(80, "活動時間は80文字以内で入力してください"),
  gender_ratio: z.string().trim().max(80, "男女比は80文字以内で入力してください"),
  average_age: z.string().trim().max(80, "平均年齢は80文字以内で入力してください"),
  recruit_age: z.string().trim().max(80, "募集年齢は80文字以内で入力してください"),
  links: z
    .array(z.string().trim().refine(httpsUrl, "httpsのURLを入力してください"))
    .max(4, "URLは4件までです"),
  body: z.string("詳細を入力してください").trim().min(1, "詳細を入力してください").max(10000, "10000文字以内"),
});

export const practiceSchema = z.object({
  venue: z.string("練習場所を入力してください").trim().min(1, "練習場所を入力してください").max(80, "練習場所は80文字以内で入力してください"),
  starts_at: z.string("練習日時を入力してください").trim().min(1, "練習日時を入力してください"),
  event: z.enum(practiceEventValues, "種目を選んでください"),
  event_other: z.string().trim().max(40, "種目名は40文字以内で入力してください"),
  capacity: z
    .number("募集人数は1〜99で入力してください")
    .int("募集人数は1〜99で入力してください")
    .min(1, "募集人数は1〜99で入力してください")
    .max(99, "募集人数は1〜99で入力してください"),
  body: z.string("詳細を入力してください").trim().min(1, "詳細を入力してください").max(2000, "詳細は2000文字以内で入力してください"),
});

export const commentSchema = z.object({
  body: z.string("コメントを入力してください").trim().min(1, "コメントを入力してください").max(2000, "2000文字以内"),
});

export const inquiryCategories = ["bug", "account", "request", "other"] as const;

export const inquiryCategoryLabels: Record<(typeof inquiryCategories)[number], string> = {
  bug: "不具合",
  account: "アカウント",
  request: "機能の要望",
  other: "その他",
};

export const inquirySchema = z.object({
  category: z.enum(inquiryCategories, "種類を選んでください"),
  email: z
    .string("メールアドレスを入力してください")
    .trim()
    .min(1, "メールアドレスを入力してください")
    .max(254, "254文字以内で入力してください")
    .email("メールアドレスを確認してください"),
  subject: z.string("件名を入力してください").trim().min(1, "件名を入力してください").max(80, "80文字以内で入力してください"),
  body: z.string("内容を入力してください").trim().min(1, "内容を入力してください").max(2000, "2000文字以内で入力してください"),
});

export const loginSchema = z.object({
  email: z.string("メールアドレスを確認してください").email("メールアドレスを確認してください"),
  password: z.string("パスワードを入力してください").min(8, "8文字以上にしてください"),
});

export const nicknameSchema = z.object({
  nickname: z.string("ニックネームを入力してください").trim().min(1, "ニックネームを入力してください").max(10, "10文字以内で入力してください"),
  nickname_public: z.boolean(),
  post_master_badge_public: z.boolean(),
});

export const resetSchema = z.object({
  email: z.string("メールアドレスを確認してください").email("メールアドレスを確認してください"),
});
