// パスワード再設定の画面を、再設定メールのリンクから来たログインだけに開くための判定。画面から切り離してテストする。

// 再設定リンクでログインしてから、新しいパスワードを決められる時間。リンク自体の有効期限(1時間)に合わせる。
export const recoveryWindowSeconds = 60 * 60;

// アクセストークン(JWT)の amr に、再設定リンクでのログイン("recovery")が、受け付ける時間内に記録されているか。
// ふつうのログイン(パスワード・Google)のセッションでは false になり、いまのパスワードを知らなくても変えられる抜け道をふさぐ。
export function isRecentRecoverySession(accessToken: string | undefined, nowSeconds = Date.now() / 1000): boolean {
  const payload = accessToken?.split(".")[1];
  if (!payload) return false;
  try {
    const padded = payload.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(payload.length / 4) * 4, "=");
    const claims = JSON.parse(atob(padded)) as { amr?: unknown };
    if (!Array.isArray(claims.amr)) return false;
    return claims.amr.some((entry: unknown) => {
      if (!entry || typeof entry !== "object") return false;
      const { method, timestamp } = entry as { method?: unknown; timestamp?: unknown };
      return method === "recovery" && typeof timestamp === "number" && nowSeconds - timestamp <= recoveryWindowSeconds;
    });
  } catch {
    return false;
  }
}
