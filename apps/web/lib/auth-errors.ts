import { L } from "./i18n";
const messages: Record<string, string> = {
  get invalid_credentials() { return L("メールアドレスまたはパスワードが違います。"); },
  get email_not_confirmed() { return L("確認メールのリンクをまだ開いていません。"); },
  get user_already_exists() { return L("このメールアドレスはすでに登録されています。"); },
  get email_exists() { return L("このメールアドレスはすでに登録されています。"); },
  get weak_password() { return L("パスワードは8文字以上にしてください。"); },
  get over_email_send_rate_limit() { return L("送信の間隔が短すぎます。しばらくしてからもう一度試してください。"); },
  get over_request_rate_limit() { return L("送信の間隔が短すぎます。しばらくしてからもう一度試してください。"); },
  get over_sms_send_rate_limit() { return L("送信の間隔が短すぎます。しばらくしてからもう一度試してください。"); },
  get signup_disabled() { return L("新規登録は現在受け付けていません。"); },
  get email_address_invalid() { return L("メールアドレスを確認してください。"); },
  get email_address_not_authorized() { return L("このメールアドレスでは登録できません。"); },
  get validation_failed() { return L("入力内容を確認してください。"); },
  get user_banned() { return L("このアカウントは利用できません。"); },
  get otp_expired() { return L("リンクの有効期限が切れています。もう一度やり直してください。"); },
  get provider_disabled() { return L("このログイン方法は現在使えません。"); },
  get email_provider_disabled() { return L("メールでのログインは現在使えません。"); },
  get bad_oauth_state() { return L("Googleログインに失敗しました。もう一度試してください。"); },
  get bad_oauth_callback() { return L("Googleログインに失敗しました。もう一度試してください。"); },
  get oauth_provider_not_supported() { return L("Googleログインに失敗しました。もう一度試してください。"); },
  get session_expired() { return L("ログインの有効期限が切れました。もう一度ログインしてください。"); },
  get refresh_token_not_found() { return L("ログインの有効期限が切れました。もう一度ログインしてください。"); },
  get captcha_failed() { return L("確認に失敗しました。もう一度試してください。"); },
  get request_timeout() { return L("処理がタイムアウトしました。もう一度試してください。"); },
  get hook_timeout() { return L("処理がタイムアウトしました。もう一度試してください。"); },
  get insufficient_aal() { return L("先に二段階認証のコードを入れてください。"); },
  get same_password() { return L("いまのパスワードとは別のパスワードにしてください。"); },
  get current_password_required() { return L("いまのパスワードを入力してください。"); },
  get current_password_invalid() { return L("いまのパスワードが違います。"); },
  get reauthentication_needed() { return L("安全のため、一度ログアウトしてログインし直してから変更してください。"); },
};

const fallback = "処理に失敗しました。もう一度試してください。";

export function authErrorMessage(error: { message?: string; code?: string }): string {
  if (error.code && messages[error.code]) return messages[error.code];
  const message = (error.message ?? "").toLowerCase();
  if (message.includes("invalid login credentials")) return messages.invalid_credentials;
  if (message.includes("email not confirmed")) return messages.email_not_confirmed;
  if (
    message.includes("already registered") ||
    message.includes("already been registered") ||
    message.includes("user already exists")
  ) {
    return messages.user_already_exists;
  }
  if (message.includes("password") && (message.includes("at least") || message.includes("weak"))) {
    return messages.weak_password;
  }
  if (
    message.includes("rate limit") ||
    message.includes("too many requests") ||
    message.includes("only request this") ||
    message.includes("for security purposes")
  ) {
    return messages.over_request_rate_limit;
  }
  if (message.includes("unable to validate email") || message.includes("invalid format")) {
    return messages.email_address_invalid;
  }
  if (message.includes("signups not allowed")) return messages.signup_disabled;
  if (message.includes("expired")) return messages.otp_expired;
  return L(fallback);
}
