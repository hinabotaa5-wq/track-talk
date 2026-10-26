import { createClient as createSupabaseClient } from "@supabase/supabase-js";
import { authErrorMessage } from "./auth-errors";

// メールアドレス・パスワードを変える前に、いまのパスワードで本人か確かめる。
// 画面のログイン(二段階認証を済ませたセッション)を置き換えないよう、保存しない別のクライアントで確かめ、
// 確かめるために作ったセッションはすぐ消す。CAPTCHA が有効なときは、そのトークンも送る。
export async function confirmCurrentPassword(email: string, password: string, captchaToken?: string): Promise<string> {
  const url = process.env.NEXT_PUBLIC_SUPABASE_URL;
  const key = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY;
  if (!url || !key) return "Supabase の公開設定がありません。";
  const probe = createSupabaseClient(url, key, {
    auth: { persistSession: false, autoRefreshToken: false, detectSessionInUrl: false },
  });
  const { error } = await probe.auth.signInWithPassword({ email, password, options: { captchaToken } });
  if (error) return authErrorMessage(error);
  await probe.auth.signOut({ scope: "local" }).catch(() => undefined);
  return "";
}
