import { z } from "zod";

export const profileSchema = z.object({
  nickname: z
    .string()
    .trim()
    .min(2, "2〜30文字で入力してください")
    .max(30, "2〜30文字で入力してください")
    .refine((v) => !/\s/.test(v), "空白は使えません"),
  bio: z.string().max(500, "500文字以内で入力してください"),
});

export const postSchema = z.object({
  title: z.string().trim().min(1, "タイトルを入力してください").max(120, "120文字以内"),
  body: z.string().trim().min(1, "本文を入力してください").max(10000, "10000文字以内"),
});

export const commentSchema = z.object({
  body: z.string().trim().min(1, "コメントを入力してください").max(2000, "2000文字以内"),
});

export const loginSchema = z.object({
  email: z.string().email("メールアドレスを確認してください"),
  password: z.string().min(8, "8文字以上にしてください"),
});

export const registerSchema = loginSchema.extend({
  nickname: profileSchema.shape.nickname,
});

export const resetSchema = z.object({
  email: z.string().email("メールアドレスを確認してください"),
});
