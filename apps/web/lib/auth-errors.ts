const messages: Record<string, string> = {
  invalid_credentials: "メールアドレスまたはパスワードが違います。",
  email_not_confirmed: "確認メールのリンクをまだ開いていません。",
  user_already_exists: "このメールアドレスはすでに登録されています。",
  email_exists: "このメールアドレスはすでに登録されています。",
  weak_password: "パスワードは8文字以上にしてください。",
  over_email_send_rate_limit: "送信の間隔が短すぎます。しばらくしてからもう一度試してください。",
  over_request_rate_limit: "送信の間隔が短すぎます。しばらくしてからもう一度試してください。",
  over_sms_send_rate_limit: "送信の間隔が短すぎます。しばらくしてからもう一度試してください。",
  signup_disabled: "新規登録は現在受け付けていません。",
  email_address_invalid: "メールアドレスを確認してください。",
  email_address_not_authorized: "このメールアドレスでは登録できません。",
  validation_failed: "入力内容を確認してください。",
  user_banned: "このアカウントは利用できません。",
  otp_expired: "リンクの有効期限が切れています。もう一度やり直してください。",
  provider_disabled: "このログイン方法は現在使えません。",
  email_provider_disabled: "メールでのログインは現在使えません。",
  bad_oauth_state: "Googleログインに失敗しました。もう一度試してください。",
  bad_oauth_callback: "Googleログインに失敗しました。もう一度試してください。",
  oauth_provider_not_supported: "Googleログインに失敗しました。もう一度試してください。",
  session_expired: "ログインの有効期限が切れました。もう一度ログインしてください。",
  refresh_token_not_found: "ログインの有効期限が切れました。もう一度ログインしてください。",
  captcha_failed: "確認に失敗しました。もう一度試してください。",
  request_timeout: "処理がタイムアウトしました。もう一度試してください。",
  hook_timeout: "処理がタイムアウトしました。もう一度試してください。",
  insufficient_aal: "先に二段階認証のコードを入れてください。",
  same_password: "いまのパスワードとは別のパスワードにしてください。",
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
  return fallback;
}
