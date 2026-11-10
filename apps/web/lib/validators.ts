import { L, type Locale } from "./i18n";
import { z } from "zod";
import { PREFECTURES } from "./prefectures";
import { appEventValues } from "./practice-events";

z.config(z.locales.ja());

// zod の既定の文言も表示言語に合わせる（本体 #530）。LocaleProvider が切り替えのたびに呼ぶ。
export function applyZodLocale(locale: Locale) {
  z.config(locale === "en" ? z.locales.en() : z.locales.ja());
}

export const postSchema = z.object({
  body: z.string({ error: () => L("詳細を入力してください") }).trim().max(10000, { error: () => L("10000文字以内") }),
});

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
  title: z.string({ error: () => L("チーム名を入力してください") }).trim().min(1, { error: () => L("チーム名を入力してください") }).max(120, { error: () => L("120文字以内") }),
  prefectures: z.array(z.string()).min(1, { error: () => L("都道府県を1つ以上選んでください") }),
  venue: z.string().trim().max(80, { error: () => L("活動場所は80文字以内で入力してください") }),
  schedule: z.string({ error: () => L("活動時間を入力してください") }).trim().min(1, { error: () => L("活動時間を入力してください") }).max(80, { error: () => L("活動時間は80文字以内で入力してください") }),
  gender_ratio: z.string().trim().max(80, { error: () => L("男女比は80文字以内で入力してください") }),
  average_age: z.string().trim().max(80, { error: () => L("平均年齢は80文字以内で入力してください") }),
  recruit_age: z.string().trim().max(80, { error: () => L("募集年齢は80文字以内で入力してください") }),
  links: z
    .array(z.string().trim().refine(httpsUrl, { error: () => L("httpsのURLを入力してください") }))
    .max(4, { error: () => L("URLは4件までです") }),
  body: z.string({ error: () => L("詳細を入力してください") }).trim().min(1, { error: () => L("詳細を入力してください") }).max(10000, { error: () => L("10000文字以内") }),
});

export const practiceSchema = z.object({
  venue: z.string({ error: () => L("練習場所を入力してください") }).trim().min(1, { error: () => L("練習場所を入力してください") }).max(80, { error: () => L("練習場所は80文字以内で入力してください") }),
  starts_at: z.string({ error: () => L("練習日時を入力してください") }).trim().min(1, { error: () => L("練習日時を入力してください") }),
  event: z.enum(appEventValues, { error: () => L("種目を選んでください") }),
  prefecture: z.enum(PREFECTURES.map((item) => item.code) as [string, ...string[]], { error: () => L("都道府県を選んでください") }),
  capacity: z
    .number({ error: () => L("募集人数は1〜99で入力してください") })
    .int({ error: () => L("募集人数は1〜99で入力してください") })
    .min(1, { error: () => L("募集人数は1〜99で入力してください") })
    .max(99, { error: () => L("募集人数は1〜99で入力してください") }),
  body: z.string({ error: () => L("詳細を入力してください") }).trim().min(1, { error: () => L("詳細を入力してください") }).max(2000, { error: () => L("詳細は2000文字以内で入力してください") }),
});

export const commentSchema = z.object({
  body: z.string({ error: () => L("コメントを入力してください") }).trim().min(1, { error: () => L("コメントを入力してください") }).max(2000, { error: () => L("2000文字以内") }),
});

// disclosure は、特定商取引法の表記で省略した住所・電話番号の開示請求。
export const inquiryCategories = ["bug", "account", "request", "other", "disclosure"] as const;

export const inquiryCategoryLabels: Record<(typeof inquiryCategories)[number], string> = {
  get bug() { return L("不具合"); },
  get account() { return L("アカウント"); },
  get request() { return L("機能の要望"); },
  get other() { return L("その他"); },
  get disclosure() { return L("特商法表記の開示請求"); },
};

export const inquirySchema = z.object({
  category: z.enum(inquiryCategories, { error: () => L("種類を選んでください") }),
  email: z
    .string({ error: () => L("メールアドレスを入力してください") })
    .trim()
    .min(1, { error: () => L("メールアドレスを入力してください") })
    .max(254, { error: () => L("254文字以内で入力してください") })
    .email({ error: () => L("メールアドレスを確認してください") }),
  subject: z.string({ error: () => L("件名を入力してください") }).trim().min(1, { error: () => L("件名を入力してください") }).max(80, { error: () => L("80文字以内で入力してください") }),
  body: z.string({ error: () => L("内容を入力してください") }).trim().min(1, { error: () => L("内容を入力してください") }).max(2000, { error: () => L("2000文字以内で入力してください") }),
});

export const loginSchema = z.object({
  email: z.string({ error: () => L("メールアドレスを確認してください") }).email({ error: () => L("メールアドレスを確認してください") }),
  password: z.string({ error: () => L("パスワードを入力してください") }).min(8, { error: () => L("8文字以上にしてください") }),
});

export const nicknameSchema = z.object({
  nickname: z.string({ error: () => L("ニックネームを入力してください") }).trim().min(1, { error: () => L("ニックネームを入力してください") }).max(10, { error: () => L("10文字以内で入力してください") }),
  nickname_public: z.boolean(),
  event: z.enum(appEventValues, { error: () => L("種目を選んでください") }),
});

export const resetSchema = z.object({
  email: z.string({ error: () => L("メールアドレスを確認してください") }).email({ error: () => L("メールアドレスを確認してください") }),
});

export const changeEmailSchema = z.object({
  email: z.string({ error: () => L("メールアドレスを確認してください") }).trim().email({ error: () => L("メールアドレスを確認してください") }).max(254, { error: () => L("254文字以内で入力してください") }),
  current: z.string({ error: () => L("いまのパスワードを入力してください") }).min(1, { error: () => L("いまのパスワードを入力してください") }),
});

export const changePasswordSchema = z
  .object({
    current: z.string({ error: () => L("いまのパスワードを入力してください") }).min(1, { error: () => L("いまのパスワードを入力してください") }),
    password: z.string({ error: () => L("パスワードを入力してください") }).min(8, { error: () => L("8文字以上にしてください") }),
    confirm: z.string({ error: () => L("もう一度入力してください") }),
  })
  .refine((v) => v.password === v.confirm, { get message() { return L("同じパスワードを入力してください"); }, path: ["confirm"] });

export const newPasswordSchema = z
  .object({
    password: z.string({ error: () => L("パスワードを入力してください") }).min(8, { error: () => L("8文字以上にしてください") }),
    confirm: z.string({ error: () => L("もう一度入力してください") }),
  })
  .refine((v) => v.password === v.confirm, { get message() { return L("同じパスワードを入力してください"); }, path: ["confirm"] });
