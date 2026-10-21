// /auth/callback に来たときの失敗の種類。ログイン画面の ?error= に入れて、出す文言を選ぶ。
// メールのリンク(登録の確認・メールアドレスの変更)も Google ログインも、同じ /auth/callback に戻ってくる。
export type CallbackError = "google" | "link-expired" | "other-browser";

// Supabase から戻ってきた URL の時点でわかる失敗。code があって error が無ければ null(交換に進む)。
export function callbackParamsError(params: URLSearchParams): CallbackError | null {
  // メールのリンクの期限切れや使用済みは、error_code=otp_expired で戻ってくる。
  if (params.get("error_code") === "otp_expired") return "link-expired";
  if (params.get("error") || params.get("error_description") || !params.get("code")) return "google";
  return null;
}

// code の交換に失敗したとき。Google ログインは同じブラウザで戻ってくるので、ここで失敗するのは
// ほとんどが、メールのリンクを登録したのとは別のブラウザ(メールアプリの中のブラウザなど)で開いたとき。
// このときも Supabase 側でメールアドレスの確認は済んでいる。
export const callbackExchangeError: CallbackError = "other-browser";

export const callbackErrorMessages: Record<string, string> = {
  config: "Supabase の公開設定がありません。",
  google: "Googleログインに失敗しました。",
  "link-expired": "メールのリンクの有効期限が切れているか、すでに使われています。ログインできないときは、もう一度登録するか、パスワードを再設定してください。",
  "other-browser":
    "このブラウザではログインを完了できませんでした。メールのリンクから来た場合、メールアドレスの確認は済んでいます。メールアドレスとパスワードでログインしてください。",
};
