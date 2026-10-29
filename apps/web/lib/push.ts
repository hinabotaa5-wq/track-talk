import { apiFetch } from "./api";
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
  if (existing && subscriptionMatchesKey(existing, publicKey, rememberedPushKey())) {
    rememberPushKey(publicKey);
    return existing.toJSON();
  }
  // 前の鍵で作った購読には、鍵を入れ替えたあと通知が届かない。外してから作り直す。
  if (existing) await existing.unsubscribe();
  const subscription = await registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: base64UrlToBytes(publicKey),
  });
  rememberPushKey(publicKey);
  return subscription.toJSON();
}

// 購読を作ったときの公開鍵。iPhone の Safari は購読の鍵を教えないので、この端末に覚えておいて比べる。
const pushKeyStorage = "tracktalk-push-key";

function rememberedPushKey(): string | null {
  try {
    return window.localStorage.getItem(pushKeyStorage);
  } catch {
    return null;
  }
}

function rememberPushKey(publicKey: string) {
  try {
    window.localStorage.setItem(pushKeyStorage, publicKey);
  } catch {
    // 保存できない端末では、次に開いたときも作り直す。
  }
}

// 購読が、いまの VAPID 公開鍵で作られたものか。ブラウザが鍵を教えないときは、この端末に覚えた鍵で比べ、
// 覚えていなければ作り直す。
export function subscriptionMatchesKey(
  subscription: Pick<PushSubscription, "options">,
  publicKey: string,
  remembered: string | null = null,
): boolean {
  const key = subscription.options?.applicationServerKey;
  if (!key) return remembered === publicKey;
  const current = new Uint8Array(key);
  const expected = base64UrlToBytes(publicKey);
  return current.length === expected.length && current.every((value, i) => value === expected[i]);
}

// VAPID の鍵を入れ替えたあと、前の鍵の購読を今の鍵で作り直して API に登録し直す。
// 鍵が変わっていなくても登録し直す。API は届かなかった宛先を消すので、消されたままにしない。
// 通知をオンにしていない端末では何もしない。
export async function refreshPushSubscription(token: string): Promise<void> {
  const existing = await currentPushSubscription();
  if (!existing) return;
  const { public_key } = await apiFetch<{ public_key: string }>("/v1/push/public-key");
  if (!public_key) return;
  if (subscriptionMatchesKey(existing, public_key, rememberedPushKey())) {
    await apiFetch<void>("/v1/me/push-subscriptions", { method: "POST", token, body: JSON.stringify(existing.toJSON()) });
    return;
  }
  await apiFetch<void>("/v1/me/push-subscriptions", {
    method: "DELETE",
    token,
    body: JSON.stringify({ endpoint: existing.endpoint }),
  }).catch(() => undefined);
  const subscription = await subscribePush(public_key);
  await apiFetch<void>("/v1/me/push-subscriptions", { method: "POST", token, body: JSON.stringify(subscription) });
}

// この端末でいま通知を受け取る設定になっているか。service worker を新しく登録はしない。
export async function currentPushSubscription(): Promise<PushSubscription | null> {
  if (!pushSupported() || Notification.permission !== "granted") return null;
  const registration = await navigator.serviceWorker.getRegistration("/");
  return (await registration?.pushManager.getSubscription()) ?? null;
}

export function monthlyLimitLabel(value: number | null): string {
  return value === null ? "上限なし" : `月${formatYen(value)}まで`;
}
