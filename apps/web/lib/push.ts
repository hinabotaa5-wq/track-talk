import { formatYen } from "./format";

// Web Push の購読まわり。通知はブラウザが対応していて、ホーム画面のアプリかブラウザで許可したときだけ届く。

export function pushSupported(): boolean {
  return (
    typeof window !== "undefined" &&
    "serviceWorker" in navigator &&
    "PushManager" in window &&
    "Notification" in window
  );
}

// VAPID の公開鍵（base64url）を applicationServerKey に渡せる形にする。
export function base64UrlToBytes(value: string): Uint8Array<ArrayBuffer> {
  const padded = value.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(value.length / 4) * 4, "=");
  const raw = atob(padded);
  const bytes = new Uint8Array(new ArrayBuffer(raw.length));
  for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
  return bytes;
}

export async function subscribePush(publicKey: string): Promise<PushSubscriptionJSON> {
  const registration = await navigator.serviceWorker.register("/sw.js", { scope: "/" });
  await navigator.serviceWorker.ready;
  const existing = await registration.pushManager.getSubscription();
  if (existing) return existing.toJSON();
  const subscription = await registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: base64UrlToBytes(publicKey),
  });
  return subscription.toJSON();
}

export function monthlyLimitLabel(value: number | null): string {
  return value === null ? "上限なし" : `月${formatYen(value)}まで`;
}
